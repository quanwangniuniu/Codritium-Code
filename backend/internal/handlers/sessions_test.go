package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"codritium/backend/internal/auth"
	"codritium/backend/internal/platform/testutil"
)

func sendSession(t *testing.T, deps SessionsDeps, handle, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/sessions", strings.NewReader(body))
	u := &auth.User{ID: uuid.New(), Handle: handle, DisplayName: handle}
	req = req.WithContext(auth.WithUser(req.Context(), u))
	rr := httptest.NewRecorder()
	PostSession(deps).ServeHTTP(rr, req)
	return rr
}

func cleanupSessions(t *testing.T, pool *pgxpool.Pool, handle string) {
	t.Helper()
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM candidate_sessions WHERE candidate_id = $1`, handle)
	})
}

func decodeSessionResp(t *testing.T, rr *httptest.ResponseRecorder) sessionResponse {
	t.Helper()
	var resp sessionResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v; body=%s", err, rr.Body.String())
	}
	return resp
}

func TestPostSession_Unauthorized(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/sessions", strings.NewReader("{}"))
	rr := httptest.NewRecorder()
	PostSession(SessionsDeps{}).ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d want 401", rr.Code)
	}
}

func TestPostSession_MissingSlug(t *testing.T) {
	pool := testutil.Pool(t)
	rr := sendSession(t, SessionsDeps{Pool: pool}, "alice", `{}`)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want 400", rr.Code)
	}
}

func TestPostSession_UnknownChallenge(t *testing.T) {
	pool := testutil.Pool(t)
	rr := sendSession(t, SessionsDeps{Pool: pool}, "alice", `{"challenge_slug":"does-not-exist"}`)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status=%d want 404", rr.Code)
	}
}

func TestPostSession_CreateThenReuse(t *testing.T) {
	pool := testutil.Pool(t)
	cleanupSessions(t, pool, "alice-reuse")
	deps := SessionsDeps{Pool: pool}

	// First call: created=true
	body := `{"challenge_slug":"22-build-rate-limiter-middleware"}`
	rr1 := sendSession(t, deps, "alice-reuse", body)
	if rr1.Code != http.StatusOK {
		t.Fatalf("first call status=%d", rr1.Code)
	}
	r1 := decodeSessionResp(t, rr1)
	if !r1.Created {
		t.Fatalf("first call should report created=true")
	}

	// Second call (same handle + slug): reused
	rr2 := sendSession(t, deps, "alice-reuse", body)
	if rr2.Code != http.StatusOK {
		t.Fatalf("second call status=%d", rr2.Code)
	}
	r2 := decodeSessionResp(t, rr2)
	if r2.Created {
		t.Fatalf("second call should report created=false (reuse)")
	}
	if r2.SessionID != r1.SessionID {
		t.Fatalf("reused session_id mismatch: %q vs %q", r2.SessionID, r1.SessionID)
	}
}

func TestPostSession_ForceNewCreatesDistinct(t *testing.T) {
	pool := testutil.Pool(t)
	cleanupSessions(t, pool, "alice-force")
	deps := SessionsDeps{Pool: pool}

	body := `{"challenge_slug":"22-build-rate-limiter-middleware"}`
	rr1 := sendSession(t, deps, "alice-force", body)
	r1 := decodeSessionResp(t, rr1)

	// force_new should yield a different session_id
	body2 := `{"challenge_slug":"22-build-rate-limiter-middleware","force_new":true}`
	rr2 := sendSession(t, deps, "alice-force", body2)
	r2 := decodeSessionResp(t, rr2)
	if !r2.Created {
		t.Fatalf("force_new should report created=true")
	}
	if r2.SessionID == r1.SessionID {
		t.Fatalf("force_new should yield a different session_id")
	}
}

func TestPostSession_IsolatedPerCandidate(t *testing.T) {
	pool := testutil.Pool(t)
	cleanupSessions(t, pool, "alice-iso")
	cleanupSessions(t, pool, "bob-iso")
	deps := SessionsDeps{Pool: pool}

	body := `{"challenge_slug":"22-build-rate-limiter-middleware"}`
	rrAlice := sendSession(t, deps, "alice-iso", body)
	rA := decodeSessionResp(t, rrAlice)

	rrBob := sendSession(t, deps, "bob-iso", body)
	rB := decodeSessionResp(t, rrBob)

	if rA.SessionID == rB.SessionID {
		t.Fatalf("bob and alice should have distinct sessions")
	}
	if !rB.Created {
		t.Fatalf("bob's first call should be created=true; existing alice session must not be reused")
	}
}
