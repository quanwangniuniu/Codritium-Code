package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"codritium/backend/internal/auth"
	"codritium/backend/internal/llm"
	"codritium/backend/internal/platform/testutil"
)

func TestSubmit_RejectsSessionOwnedBySomeoneElse(t *testing.T) {
	pool := testutil.Pool(t)
	ctx := context.Background()

	owner := testutil.NewUser(t, pool, "user")
	sid := testutil.NewSession(t, pool, owner, "22-build-rate-limiter-middleware", "medium")

	var builds int
	reg := llm.NewAgentRegistry(func(context.Context, uuid.UUID, string) (*llm.Agent, error) {
		builds++
		return &llm.Agent{}, nil
	})
	if _, err := reg.GetOrCreate(ctx, sid, "slug"); err != nil {
		t.Fatalf("GetOrCreate: %v", err)
	}

	body := `{"problem_slug":"22-build-rate-limiter-middleware","session_id":"` + sid.String() + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/submissions", strings.NewReader(body))
	intruder := testutil.NewUser(t, pool, "user")
	req = req.WithContext(auth.WithUser(req.Context(), intruder))
	rr := httptest.NewRecorder()
	Submit(SubmissionDeps{Pool: pool, Agents: reg}).ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s, want 404", rr.Code, rr.Body.String())
	}
	var submitted *string
	if err := pool.QueryRow(ctx,
		`SELECT submitted_at::text FROM candidate_sessions WHERE session_id = $1`, sid,
	).Scan(&submitted); err != nil {
		t.Fatalf("read session: %v", err)
	}
	if submitted != nil {
		t.Fatalf("foreign submit marked the session submitted at %s", *submitted)
	}
	if _, err := reg.GetOrCreate(ctx, sid, "slug"); err != nil {
		t.Fatalf("GetOrCreate: %v", err)
	}
	if builds != 1 {
		t.Fatalf("owner's agent was dropped by a foreign submit (builds=%d)", builds)
	}
}
