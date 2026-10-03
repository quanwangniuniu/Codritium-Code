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

	owner := "owner-" + uuid.NewString()
	var sid uuid.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO candidate_sessions (candidate_id, challenge_id, difficulty)
		VALUES ($1, '22-build-rate-limiter-middleware', 'medium')
		RETURNING session_id`, owner).Scan(&sid); err != nil {
		t.Fatalf("create session: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM candidate_sessions WHERE session_id = $1`, sid)
	})

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
	intruder := &auth.User{ID: uuid.New(), Handle: "intruder-" + uuid.NewString()}
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

func TestSubmit_ReusesSubmissionForSameSession(t *testing.T) {
	pool := testutil.Pool(t)
	ctx := context.Background()

	user := &auth.User{
		ID:          uuid.New(),
		Handle:      "submit-retry-" + uuid.NewString(),
		DisplayName: "Submit Retry",
	}

	if _, err := pool.Exec(ctx, `
		INSERT INTO users (id, handle, display_name, role)
		VALUES ($1, $2, $3, 'user')`,
		user.ID,
		user.Handle,
		user.DisplayName,
	); err != nil {
		t.Fatalf("create user: %v", err)
	}

	var sessionID uuid.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO candidate_sessions (
			candidate_id,
			challenge_id,
			difficulty
		)
		VALUES (
			$1,
			'22-build-rate-limiter-middleware',
			'medium'
		)
		RETURNING session_id`,
		user.Handle,
	).Scan(&sessionID); err != nil {
		t.Fatalf("create session: %v", err)
	}

	var problemID uuid.UUID
	if err := pool.QueryRow(ctx, `
		SELECT id
		FROM problems
		WHERE slug = '22-build-rate-limiter-middleware'`,
	).Scan(&problemID); err != nil {
		t.Fatalf("find problem: %v", err)
	}

	var existingSubmissionID uuid.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO submissions (
			user_id,
			problem_id,
			variant,
			code_files,
			session_id,
			status
		)
		VALUES ($1, $2, 'as-is', '{}'::jsonb, $3, 'grading')
		RETURNING id`,
		user.ID,
		problemID,
		sessionID,
	).Scan(&existingSubmissionID); err != nil {
		t.Fatalf("create existing submission: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(
			context.Background(),
			`DELETE FROM submissions WHERE id = $1`,
			existingSubmissionID,
		)
		_, _ = pool.Exec(
			context.Background(),
			`DELETE FROM candidate_sessions WHERE session_id = $1`,
			sessionID,
		)
		_, _ = pool.Exec(
			context.Background(),
			`DELETE FROM users WHERE id = $1`,
			user.ID,
		)
	})

	body := `{
		"problem_slug": "22-build-rate-limiter-middleware",
		"session_id": "` + sessionID.String() + `",
		"code_files": {"solution.py": "updated"}
	}`

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/submissions",
		strings.NewReader(body),
	)
	req = req.WithContext(auth.WithUser(req.Context(), user))

	rr := httptest.NewRecorder()
	Submit(SubmissionDeps{Pool: pool}).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf(
			"status = %d, body = %s, want 200",
			rr.Code,
			rr.Body.String(),
		)
	}

	if !strings.Contains(rr.Body.String(), existingSubmissionID.String()) {
		t.Fatalf(
			"response does not contain existing submission ID: %s",
			rr.Body.String(),
		)
	}
	if !strings.Contains(rr.Body.String(), `"reused":true`) {
		t.Fatalf(
			"response does not mark submission as reused: %s",
			rr.Body.String(),
		)
	}

	var count int
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM submissions
		WHERE session_id = $1`,
		sessionID,
	).Scan(&count); err != nil {
		t.Fatalf("count submissions: %v", err)
	}

	if count != 1 {
		t.Fatalf("submission count = %d, want 1", count)
	}
}
