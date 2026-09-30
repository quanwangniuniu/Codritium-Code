package comments

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
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
	u := testutil.NewUser(t, pool, "user")

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
		Handler{Pool: pool}.createComment(rr, req)
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

func TestListComments_KeysetPagination(t *testing.T) {
	pool := testutil.Pool(t)
	ctx := context.Background()
	u := testutil.NewUser(t, pool, "admin") // admins skip the graded gate
	slug := "zz-pagination-" + u.ID.String()[:8]
	for i := 0; i < 3; i++ {
		if _, err := pool.Exec(ctx, `
			INSERT INTO comments (problem_slug, user_id, body, created_at)
			VALUES ($1, $2, $3, now() - make_interval(mins => $4))`,
			slug, u.ID, fmt.Sprintf("c%d", i), i); err != nil {
			t.Fatal(err)
		}
	}
	h := Handler{Pool: pool}
	type page struct {
		Comments []struct {
			Body string `json:"body"`
		} `json:"comments"`
		Next string `json:"next_cursor"`
	}
	list := func(q string) page {
		rr := testutil.Call(t, http.HandlerFunc(h.listComments), "GET", "/?"+q, map[string]string{"slug": slug}, u, "")
		testutil.WantStatus(t, rr, http.StatusOK)
		return testutil.Decode[page](t, rr)
	}
	p1 := list("limit=2")
	if len(p1.Comments) != 2 || p1.Comments[0].Body != "c0" || p1.Next == "" {
		t.Fatalf("page1=%+v", p1)
	}
	p2 := list("limit=2&cursor=" + url.QueryEscape(p1.Next))
	if len(p2.Comments) != 1 || p2.Comments[0].Body != "c2" || p2.Next != "" {
		t.Fatalf("page2=%+v", p2)
	}
}
