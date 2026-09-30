package agent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"codritium/backend/internal/auth"
	"codritium/backend/internal/events"
	"codritium/backend/internal/platform/testutil"
)

func createSessionFor(t *testing.T, pool *pgxpool.Pool, u *auth.User) uuid.UUID {
	t.Helper()
	return testutil.NewSession(t, pool, u, "22-build-rate-limiter-middleware", "medium")
}

func sendEvent(t *testing.T, deps Handler, u *auth.User, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/events", strings.NewReader(body))
	req = req.WithContext(auth.WithUser(req.Context(), u))
	rr := httptest.NewRecorder()
	http.HandlerFunc(deps.postEvent).ServeHTTP(rr, req)
	return rr
}

func TestPostEvent_Unauthorized(t *testing.T) {
	deps := Handler{Pool: nil, Events: nil}
	req := httptest.NewRequest(http.MethodPost, "/api/events", strings.NewReader("{}"))
	rr := httptest.NewRecorder()
	http.HandlerFunc(deps.postEvent).ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d want 401", rr.Code)
	}
}

func TestPostEvent_BadJSON(t *testing.T) {
	pool := testutil.Pool(t)
	alice := testutil.NewUser(t, pool, "user")
	deps := Handler{Pool: pool, Events: events.NewStore(pool)}
	rr := sendEvent(t, deps, alice, `{not json`)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want 400", rr.Code)
	}
}

func TestPostEvent_BadSessionID(t *testing.T) {
	pool := testutil.Pool(t)
	alice := testutil.NewUser(t, pool, "user")
	deps := Handler{Pool: pool, Events: events.NewStore(pool)}
	rr := sendEvent(t, deps, alice, `{"session_id":"not-uuid","kind":"ai_output_read","payload":{}}`)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want 400", rr.Code)
	}
}

func TestPostEvent_BackendKindRejected(t *testing.T) {
	pool := testutil.Pool(t)
	alice := testutil.NewUser(t, pool, "user")
	sid := createSessionFor(t, pool, alice)
	deps := Handler{Pool: pool, Events: events.NewStore(pool)}
	body := `{"session_id":"` + sid.String() + `","kind":"tool_use_proposed","payload":{}}`
	rr := sendEvent(t, deps, alice, body)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want 400 for backend-only kind", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), `"error":"kind_not_allowed"`) {
		t.Fatalf("body did not mention disallowed kind: %s", rr.Body.String())
	}
}

func TestPostEvent_UnknownSession(t *testing.T) {
	pool := testutil.Pool(t)
	alice := testutil.NewUser(t, pool, "user")
	deps := Handler{Pool: pool, Events: events.NewStore(pool)}
	body := `{"session_id":"` + uuid.NewString() + `","kind":"ai_output_read","payload":{"message_id":"m1","pause_duration_sec":3,"scroll_depth_percent":80,"next_action_kind":"new_prompt"}}`
	rr := sendEvent(t, deps, alice, body)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status=%d want 404", rr.Code)
	}
}

func TestPostEvent_CrossUserReturns404(t *testing.T) {
	pool := testutil.Pool(t)
	alice := testutil.NewUser(t, pool, "user")
	bob := testutil.NewUser(t, pool, "user")
	sid := createSessionFor(t, pool, alice)
	deps := Handler{Pool: pool, Events: events.NewStore(pool)}
	// bob tries to emit on alice's session
	body := `{"session_id":"` + sid.String() + `","kind":"ai_output_read","payload":{"message_id":"m1","pause_duration_sec":3,"scroll_depth_percent":80,"next_action_kind":"new_prompt"}}`
	rr := sendEvent(t, deps, bob, body)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status=%d want 404 (no existence leak)", rr.Code)
	}
}

func TestPostEvent_AIOutputReadHappy(t *testing.T) {
	pool := testutil.Pool(t)
	alice := testutil.NewUser(t, pool, "user")
	sid := createSessionFor(t, pool, alice)
	deps := Handler{Pool: pool, Events: events.NewStore(pool)}
	body := `{
		"session_id":"` + sid.String() + `",
		"kind":"ai_output_read",
		"payload":{
			"message_id":"msg-3-2",
			"pause_duration_sec":47,
			"scroll_depth_percent":100,
			"next_action_kind":"new_prompt"
		}
	}`
	rr := sendEvent(t, deps, alice, body)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("status=%d want 204; body=%s", rr.Code, rr.Body.String())
	}

	// verify persisted
	var count int
	err := pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM session_events WHERE session_id=$1 AND kind='ai_output_read'`,
		sid,
	).Scan(&count)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("count=%d want 1", count)
	}
}

func TestPostEvent_CandidateRevertedEditHappy(t *testing.T) {
	pool := testutil.Pool(t)
	alice := testutil.NewUser(t, pool, "user")
	sid := createSessionFor(t, pool, alice)
	deps := Handler{Pool: pool, Events: events.NewStore(pool)}
	body := `{
		"session_id":"` + sid.String() + `",
		"kind":"candidate_reverted_edit",
		"payload":{
			"original_tool_use_id":"tu-3-1",
			"revert_method":"undo",
			"files_affected":["limiter_store.py"],
			"next_action_kind":"new_prompt"
		}
	}`
	rr := sendEvent(t, deps, alice, body)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("status=%d want 204; body=%s", rr.Code, rr.Body.String())
	}
}

func TestPostEvent_UnknownPayloadFieldRejected(t *testing.T) {
	pool := testutil.Pool(t)
	alice := testutil.NewUser(t, pool, "user")
	sid := createSessionFor(t, pool, alice)
	deps := Handler{Pool: pool, Events: events.NewStore(pool)}
	body := `{
		"session_id":"` + sid.String() + `",
		"kind":"ai_output_read",
		"payload":{
			"message_id":"m1",
			"pause_duration_sec":3,
			"scroll_depth_percent":80,
			"next_action_kind":"new_prompt",
			"smuggled_field":"oops"
		}
	}`
	rr := sendEvent(t, deps, alice, body)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want 400 for unknown field", rr.Code)
	}
}
