package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"codritium/backend/internal/auth"
	"codritium/backend/internal/platform/testutil"
)

// Commenting on a problem unlocks once the user has a graded submission.
// Regression: the gate used candidate_sessions.graded_at, which nothing
// ever set, so non-admins were locked out forever.
func TestCreateComment_UnlocksAfterGradedSubmission(t *testing.T) {
	pool := testutil.Pool(t)
	ctx := context.Background()
	u := newForumUser(t, pool, "user")

	var problemID, slug string
	if err := pool.QueryRow(ctx,
		`SELECT id::text, slug FROM problems WHERE status = 'published' ORDER BY slug LIMIT 1`,
	).Scan(&problemID, &slug); err != nil {
		t.Skipf("needs a seeded problem: %v", err)
	}

	post := func() int {
		req := httptest.NewRequest(http.MethodPost, "/api/problems/"+slug+"/comments",
			strings.NewReader(`{"body":"The failing test pointed straight at the bug."}`))
		req.SetPathValue("slug", slug)
		req = req.WithContext(auth.WithUser(req.Context(), u))
		rr := httptest.NewRecorder()
		CreateComment(CommentsDeps{Pool: pool}).ServeHTTP(rr, req)
		return rr.Code
	}

	if got := post(); got != http.StatusForbidden {
		t.Fatalf("before grading: status=%d want 403", got)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO submissions (user_id, problem_id, status, final_score, graded_at)
		VALUES ($1, $2, 'graded', 50, now())`, u.ID, problemID); err != nil {
		t.Fatal(err)
	}
	if got := post(); got != http.StatusCreated {
		t.Fatalf("after grading: status=%d want 201", got)
	}
}
