package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"codritium/backend/internal/auth"
	"codritium/backend/internal/events"
)

func newEventsTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://codritium:codritium@localhost:5434/codritium?sslmode=disable"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Skipf("no postgres at %s: %v", dsn, err)
	}
	if err := pool.Ping(ctx); err != nil {
		t.Skipf("postgres ping: %v", err)
	}
	return pool
}

func createSessionFor(t *testing.T, pool *pgxpool.Pool, handle string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := pool.QueryRow(context.Background(), `
		INSERT INTO candidate_sessions (candidate_id, challenge_id, difficulty)
		VALUES ($1, $2, 'medium')
		RETURNING session_id`,
		handle, "22-build-rate-limiter-middleware",
	).Scan(&id)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM candidate_sessions WHERE session_id = $1`, id)
	})
	return id
}

func sendEvent(t *testing.T, deps EventsDeps, handle, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/events", strings.NewReader(body))
	u := &auth.User{ID: uuid.New(), Handle: handle, DisplayName: handle}
	req = req.WithContext(auth.WithUser(req.Context(), u))
	rr := httptest.NewRecorder()
	PostEvent(deps).ServeHTTP(rr, req)
	return rr
}

func TestPostEvent_Unauthorized(t *testing.T) {
	deps := EventsDeps{Pool: nil, Events: nil}
	req := httptest.NewRequest(http.MethodPost, "/api/events", strings.NewReader("{}"))
	rr := httptest.NewRecorder()
	PostEvent(deps).ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d want 401", rr.Code)
	}
}

func TestPostEvent_BadJSON(t *testing.T) {
	pool := newEventsTestPool(t)
	deps := EventsDeps{Pool: pool, Events: events.NewStore(pool)}
	rr := sendEvent(t, deps, "alice", `{not json`)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want 400", rr.Code)
	}
}

func TestPostEvent_BadSessionID(t *testing.T) {
	pool := newEventsTestPool(t)
	deps := EventsDeps{Pool: pool, Events: events.NewStore(pool)}
	rr := sendEvent(t, deps, "alice", `{"session_id":"not-uuid","kind":"ai_output_read","payload":{}}`)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want 400", rr.Code)
	}
}

func TestPostEvent_BackendKindRejected(t *testing.T) {
	pool := newEventsTestPool(t)
	sid := createSessionFor(t, pool, "alice")
	deps := EventsDeps{Pool: pool, Events: events.NewStore(pool)}
	body := `{"session_id":"` + sid.String() + `","kind":"tool_use_proposed","payload":{}}`
	rr := sendEvent(t, deps, "alice", body)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want 400 for backend-only kind", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "kind not allowed") {
		t.Fatalf("body did not mention disallowed kind: %s", rr.Body.String())
	}
}

func TestPostEvent_UnknownSession(t *testing.T) {
	pool := newEventsTestPool(t)
	deps := EventsDeps{Pool: pool, Events: events.NewStore(pool)}
	body := `{"session_id":"` + uuid.NewString() + `","kind":"ai_output_read","payload":{"message_id":"m1","pause_duration_sec":3,"scroll_depth_percent":80,"next_action_kind":"new_prompt"}}`
	rr := sendEvent(t, deps, "alice", body)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status=%d want 404", rr.Code)
	}
}

func TestPostEvent_CrossUserReturns404(t *testing.T) {
	pool := newEventsTestPool(t)
	sid := createSessionFor(t, pool, "alice")
	deps := EventsDeps{Pool: pool, Events: events.NewStore(pool)}
	// bob tries to emit on alice's session
	body := `{"session_id":"` + sid.String() + `","kind":"ai_output_read","payload":{"message_id":"m1","pause_duration_sec":3,"scroll_depth_percent":80,"next_action_kind":"new_prompt"}}`
	rr := sendEvent(t, deps, "bob", body)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status=%d want 404 (no existence leak)", rr.Code)
	}
}

func TestPostEvent_AIOutputReadHappy(t *testing.T) {
	pool := newEventsTestPool(t)
	sid := createSessionFor(t, pool, "alice")
	deps := EventsDeps{Pool: pool, Events: events.NewStore(pool)}
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
	rr := sendEvent(t, deps, "alice", body)
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
	pool := newEventsTestPool(t)
	sid := createSessionFor(t, pool, "alice")
	deps := EventsDeps{Pool: pool, Events: events.NewStore(pool)}
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
	rr := sendEvent(t, deps, "alice", body)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("status=%d want 204; body=%s", rr.Code, rr.Body.String())
	}
}

func TestPostEvent_UnknownPayloadFieldRejected(t *testing.T) {
	pool := newEventsTestPool(t)
	sid := createSessionFor(t, pool, "alice")
	deps := EventsDeps{Pool: pool, Events: events.NewStore(pool)}
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
	rr := sendEvent(t, deps, "alice", body)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want 400 for unknown field", rr.Code)
	}
}
