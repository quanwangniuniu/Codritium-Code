package handlers

import (
	"context"
	"net/http"
	"testing"
	"time"

	"codritium/backend/internal/auth"
	"codritium/backend/internal/platform/testutil"
)

func day(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

func TestStreaks(t *testing.T) {
	start, today := day("2026-09-01"), day("2026-09-30")
	cases := []struct {
		name             string
		active           []string
		current, longest int
	}{
		{"none", nil, 0, 0},
		{"today only", []string{"2026-09-30"}, 1, 1},
		{"ends yesterday still counts", []string{"2026-09-28", "2026-09-29"}, 2, 2},
		{"broken two days ago", []string{"2026-09-27", "2026-09-28"}, 0, 2},
		{"longest earlier", []string{"2026-09-02", "2026-09-03", "2026-09-04", "2026-09-30"}, 1, 3},
	}
	for _, c := range cases {
		m := map[string]int{}
		for _, d := range c.active {
			m[d] = 1
		}
		cur, long := streaks(m, start, today)
		if cur != c.current || long != c.longest {
			t.Errorf("%s: got current=%d longest=%d, want %d/%d", c.name, cur, long, c.current, c.longest)
		}
	}
}

func TestApplyProfileUpdate(t *testing.T) {
	str := func(s string) *string { return &s }
	long := func(n int) *string {
		b := make([]rune, n)
		for i := range b {
			b[i] = '字'
		}
		s := string(b)
		return &s
	}
	cases := []struct {
		name    string
		req     updateMeRequest
		wantErr string
	}{
		{"trims and sets", updateMeRequest{DisplayName: str("  Ada  "), Bio: str(" hi "), Region: str("AU")}, ""},
		{"empty name rejected", updateMeRequest{DisplayName: str("   ")}, "invalid_display_name"},
		{"name at limit ok", updateMeRequest{DisplayName: long(maxDisplayNameRunes)}, ""},
		{"name over limit", updateMeRequest{DisplayName: long(maxDisplayNameRunes + 1)}, "invalid_display_name"},
		{"bio over limit", updateMeRequest{Bio: long(maxBioRunes + 1)}, "invalid_bio"},
		{"region over limit", updateMeRequest{Region: long(maxRegionRunes + 1)}, "invalid_region"},
		{"clear bio", updateMeRequest{Bio: str("")}, ""},
	}
	for _, c := range cases {
		u := &auth.User{DisplayName: "old", Bio: "old bio", Region: "US"}
		err := applyProfileUpdate(u, c.req)
		got := ""
		if err != nil {
			got = err.Error()
		}
		if got != c.wantErr {
			t.Errorf("%s: err=%q want %q", c.name, got, c.wantErr)
		}
	}
	u := &auth.User{DisplayName: "old", Bio: "keep", Region: "US"}
	_ = applyProfileUpdate(u, updateMeRequest{DisplayName: str("  Ada  ")})
	if u.DisplayName != "Ada" || u.Bio != "keep" || u.Region != "US" {
		t.Errorf("partial update changed wrong fields: %+v", u)
	}
}

func TestProfile_RequiresLogin(t *testing.T) {
	deps := ProfileDeps{}
	for _, h := range []http.HandlerFunc{GetMyProfile(deps), UpdateMe(deps)} {
		rr := forumCall(t, h, http.MethodGet, "/", "", nil, "")
		wantStatus(t, rr, http.StatusUnauthorized)
	}
}

func TestProfile_Aggregates(t *testing.T) {
	pool := testutil.Pool(t)
	ctx := context.Background()
	u := newForumUser(t, pool, "user")
	now := day("2026-09-30").Add(15 * time.Hour)

	var slugs []string
	var ids []string
	rows, err := pool.Query(ctx, `SELECT id::text, slug FROM problems WHERE status = 'published' ORDER BY slug LIMIT 3`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var id, slug string
		if err := rows.Scan(&id, &slug); err != nil {
			t.Fatal(err)
		}
		ids, slugs = append(ids, id), append(slugs, slug)
	}
	rows.Close()
	if len(ids) < 3 {
		t.Skip("needs at least 3 seeded problems")
	}

	insert := func(problemID, status string, score *float64, at time.Time, scores string) {
		t.Helper()
		if _, err := pool.Exec(ctx, `
			INSERT INTO submissions (user_id, problem_id, status, final_score, scores, submitted_at, graded_at)
			VALUES ($1, $2, $3, $4, $5::jsonb, $6, $6)`,
			u.ID, problemID, status, score, scores, at); err != nil {
			t.Fatalf("insert submission: %v", err)
		}
	}
	f := func(v float64) *float64 { return &v }
	dims := `{"dimension_scores":{"correctness":{"score":4},"communication":{"score":2},"verification":{"score":null}}}`
	// Problem 0: solved (85) after a failing attempt. Problem 1: graded below
	// the threshold, so attempting. Problem 2: only opened in the workspace.
	insert(ids[0], "graded", f(40), now.AddDate(0, 0, -2), dims)
	insert(ids[0], "graded", f(85), now.AddDate(0, 0, -1), dims)
	insert(ids[1], "graded", f(59.5), now, `{}`)
	if _, err := pool.Exec(ctx,
		`INSERT INTO candidate_sessions (candidate_id, challenge_id, difficulty) VALUES ($1, $2, 'easy')`, u.Handle, slugs[2]); err != nil {
		t.Fatalf("insert session: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM candidate_sessions WHERE candidate_id = $1`, u.Handle)
	})

	// One public post (upvoted) and one anonymous post that must not count.
	if _, err := pool.Exec(ctx, `
		INSERT INTO forum_posts (user_id, section, title, body_md, upvotes, created_at)
		VALUES ($1, 'career', 'public', 'b', 3, $2),
		       ($1, 'interview', 'anon', 'b', 9, $2)`, u.ID, now); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE forum_posts SET is_anonymous = true WHERE user_id = $1 AND title = 'anon'`, u.ID); err != nil {
		t.Fatal(err)
	}

	p, err := buildProfile(ctx, pool, u, now)
	if err != nil {
		t.Fatalf("buildProfile: %v", err)
	}
	if p.Solved.Solved != 1 || p.Solved.Attempting != 2 {
		t.Errorf("solved=%d attempting=%d, want 1/2", p.Solved.Solved, p.Solved.Attempting)
	}
	if len(p.SolvedProblems) != 1 || p.SolvedProblems[0].Slug != slugs[0] || p.SolvedProblems[0].BestScore != 85 {
		t.Errorf("solved problems = %+v", p.SolvedProblems)
	}
	if p.Calendar.TotalSubmissions != 3 || p.Calendar.ActiveDays != 3 || p.Calendar.CurrentStreak != 3 || p.Calendar.MaxStreak != 3 {
		t.Errorf("calendar = %+v", p.Calendar)
	}
	byDim := map[string]dimensionAverage{}
	for _, d := range p.Dimensions {
		byDim[d.Dimension] = d
	}
	if d := byDim["correctness"]; d.Average == nil || *d.Average != 4 || d.Samples != 2 {
		t.Errorf("correctness = %+v", d)
	}
	if d := byDim["verification"]; d.Average != nil {
		t.Errorf("verification should have no data, got %+v", d)
	}
	if len(p.Dimensions) != len(profileDimensions) {
		t.Errorf("want %d dimensions, got %d", len(profileDimensions), len(p.Dimensions))
	}
	if p.BestScore == nil || *p.BestScore != 85 {
		t.Errorf("best score = %v", p.BestScore)
	}
	if p.Community.Posts.Total != 1 || p.Community.Upvotes.Total != 3 || len(p.RecentPosts) != 1 {
		t.Errorf("community = %+v posts=%d (anonymous post leaked?)", p.Community, len(p.RecentPosts))
	}
	if len(p.RecentSubmissions) != 3 {
		t.Errorf("recent submissions = %d", len(p.RecentSubmissions))
	}
}

func TestUpdateMe_Persists(t *testing.T) {
	pool := testutil.Pool(t)
	u := newForumUser(t, pool, "user")
	deps := ProfileDeps{Pool: pool}

	rr := forumCall(t, UpdateMe(deps), http.MethodPatch, "/api/me", "", u,
		`{"display_name":"  Grace  ","bio":"Debugs things","region":"NZ"}`)
	wantStatus(t, rr, http.StatusOK)
	var name, bio, region string
	if err := pool.QueryRow(context.Background(),
		`SELECT display_name, COALESCE(bio,''), COALESCE(region,'') FROM users WHERE id = $1`, u.ID).
		Scan(&name, &bio, &region); err != nil {
		t.Fatal(err)
	}
	if name != "Grace" || bio != "Debugs things" || region != "NZ" {
		t.Errorf("persisted %q %q %q", name, bio, region)
	}

	rr = forumCall(t, UpdateMe(deps), http.MethodPatch, "/api/me", "", u, `{"role":"admin"}`)
	wantStatus(t, rr, http.StatusBadRequest)
	rr = forumCall(t, UpdateMe(deps), http.MethodPatch, "/api/me", "", u, `{"display_name":""}`)
	wantStatus(t, rr, http.StatusBadRequest)
}
