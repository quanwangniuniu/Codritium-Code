// Package profile builds the LeetCode-style profile read model.
package profile

import (
	"context"
	"net/http"
	"sort"
	"time"

	"codritium/backend/internal/platform/httpx"

	"codritium/backend/internal/problems"

	"github.com/jackc/pgx/v5/pgxpool"

	"codritium/backend/internal/auth"
)

// SolvedScoreThreshold is the minimum final_score (0-100) for a graded
// submission to count its problem as solved.
const SolvedScoreThreshold = 60

// profileDimensions are the grader's rubric dimensions in display order.
var profileDimensions = []string{
	"correctness", "problem_decomposition", "ai_collaboration", "verification", "communication",
}

// Routes registers the profile API.
func (h Handler) Routes(rt *httpx.Router) {
	rt.Handle("GET /api/me/profile", h.getMyProfile)
}

type Handler struct {
	Pool *pgxpool.Pool
	// Now is injectable so streak math is testable; nil means time.Now.
	Now func() time.Time
}

type profileUser struct {
	Handle      string    `json:"handle"`
	DisplayName string    `json:"display_name"`
	Bio         string    `json:"bio"`
	Region      string    `json:"region"`
	Tier        string    `json:"tier"`
	AvatarURL   string    `json:"avatar_url"`
	AvatarColor string    `json:"avatar_color"`
	MemberSince time.Time `json:"member_since"`
}

type solvedBucket struct {
	Key    string `json:"key"`
	Solved int    `json:"solved"`
	Total  int    `json:"total"`
}

type profileSolved struct {
	Solved       int            `json:"solved"`
	Total        int            `json:"total"`
	Attempting   int            `json:"attempting"`
	ByDifficulty []solvedBucket `json:"by_difficulty"`
	ByCategory   []solvedBucket `json:"by_category"`
}

type calendarDay struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
}

type profileCalendar struct {
	Days             []calendarDay `json:"days"`
	TotalSubmissions int           `json:"total_submissions"`
	ActiveDays       int           `json:"active_days"`
	CurrentStreak    int           `json:"current_streak"`
	MaxStreak        int           `json:"max_streak"`
}

type dimensionAverage struct {
	Dimension string   `json:"dimension"`
	Average   *float64 `json:"average"`
	Samples   int      `json:"samples"`
}

type countWithWeek struct {
	Total    int `json:"total"`
	LastWeek int `json:"last_week"`
}

type profileCommunity struct {
	Posts    countWithWeek `json:"posts"`
	Comments countWithWeek `json:"comments"`
	Upvotes  countWithWeek `json:"upvotes"`
}

type profileSubmission struct {
	ID           string    `json:"id"`
	ProblemSlug  string    `json:"problem_slug"`
	ProblemTitle string    `json:"problem_title"`
	Difficulty   string    `json:"difficulty"`
	Status       string    `json:"status"`
	FinalScore   *float64  `json:"final_score"`
	SubmittedAt  time.Time `json:"submitted_at"`
}

type profileSolvedProblem struct {
	Slug       string    `json:"slug"`
	Title      string    `json:"title"`
	Category   string    `json:"category"`
	Difficulty string    `json:"difficulty"`
	BestScore  float64   `json:"best_score"`
	SolvedAt   time.Time `json:"solved_at"`
}

type profilePost struct {
	ID           string    `json:"id"`
	Title        string    `json:"title"`
	Section      string    `json:"section"`
	Upvotes      int       `json:"upvotes"`
	CommentCount int       `json:"comment_count"`
	CreatedAt    time.Time `json:"created_at"`
}

type profileResponse struct {
	User              profileUser            `json:"user"`
	Solved            profileSolved          `json:"solved"`
	Calendar          profileCalendar        `json:"calendar"`
	Dimensions        []dimensionAverage     `json:"dimensions"`
	BestScore         *float64               `json:"best_score"`
	Community         profileCommunity       `json:"community"`
	RecentSubmissions []profileSubmission    `json:"recent_submissions"`
	SolvedProblems    []profileSolvedProblem `json:"solved_problems"`
	RecentPosts       []profilePost          `json:"recent_posts"`
}

// GetMyProfile serves the signed-in user's profile. GET /api/me/profile
func (h Handler) getMyProfile(w http.ResponseWriter, r *http.Request) {
	u := auth.FromContext(r.Context())
	if u == nil {
		httpx.JSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
		return
	}
	now := time.Now
	if h.Now != nil {
		now = h.Now
	}
	p, err := buildProfile(r.Context(), h.Pool, u, now().UTC())
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, p)
}

// buildProfile assembles every profile section for one user. It only reads
// data that is safe to show publicly (anonymous forum posts are excluded), so
// a future public /users/{handle} route can reuse it.
func buildProfile(ctx context.Context, pool *pgxpool.Pool, u *auth.User, now time.Time) (*profileResponse, error) {
	p := &profileResponse{
		User: profileUser{
			Handle: u.Handle, DisplayName: u.DisplayName, Bio: u.Bio, Region: u.Region,
			Tier: u.Tier, AvatarURL: u.AvatarURL, AvatarColor: u.AvatarColor,
		},
		RecentSubmissions: []profileSubmission{},
		SolvedProblems:    []profileSolvedProblem{},
		RecentPosts:       []profilePost{},
	}
	if err := pool.QueryRow(ctx, `SELECT COALESCE(created_at, now()) FROM users WHERE id = $1`, u.ID).
		Scan(&p.User.MemberSince); err != nil {
		return nil, err
	}
	steps := []func() error{
		func() error { return loadSolved(ctx, pool, u, p) },
		func() error { return loadCalendar(ctx, pool, u, now, p) },
		func() error { return loadDimensions(ctx, pool, u, p) },
		func() error { return loadCommunity(ctx, pool, u, now, p) },
		func() error { return loadRecent(ctx, pool, u, p) },
	}
	for _, step := range steps {
		if err := step(); err != nil {
			return nil, err
		}
	}
	return p, nil
}

func loadSolved(ctx context.Context, pool *pgxpool.Pool, u *auth.User, p *profileResponse) error {
	// Totals per difficulty / category over the published catalog.
	diffTotals, catTotals, err := problems.Store{Pool: pool}.CatalogTotals(ctx)
	if err != nil {
		return err
	}
	for _, n := range diffTotals {
		p.Solved.Total += n
	}

	rows, err := pool.Query(ctx, `
		SELECT pr.slug, pr.title, pr.category, pr.difficulty,
		       MAX(s.final_score)::float8, MIN(s.graded_at)
		FROM submissions s
		JOIN problems pr ON pr.id = s.problem_id
		WHERE s.user_id = $1 AND s.status = 'graded' AND s.final_score >= $2
		GROUP BY pr.slug, pr.title, pr.category, pr.difficulty
		ORDER BY MIN(s.graded_at) DESC`, u.ID, SolvedScoreThreshold)
	if err != nil {
		return err
	}
	diffSolved := map[string]int{}
	catSolved := map[string]int{}
	solvedSlugs := map[string]bool{}
	for rows.Next() {
		var sp profileSolvedProblem
		var solvedAt *time.Time
		if err := rows.Scan(&sp.Slug, &sp.Title, &sp.Category, &sp.Difficulty, &sp.BestScore, &solvedAt); err != nil {
			rows.Close()
			return err
		}
		if solvedAt != nil {
			sp.SolvedAt = *solvedAt
		}
		p.SolvedProblems = append(p.SolvedProblems, sp)
		diffSolved[sp.Difficulty]++
		catSolved[sp.Category]++
		solvedSlugs[sp.Slug] = true
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	p.Solved.Solved = len(p.SolvedProblems)

	for _, d := range []string{"easy", "medium", "hard"} {
		p.Solved.ByDifficulty = append(p.Solved.ByDifficulty,
			solvedBucket{Key: d, Solved: diffSolved[d], Total: diffTotals[d]})
	}
	cats := make([]string, 0, len(catTotals))
	for c := range catTotals {
		cats = append(cats, c)
	}
	sort.Slice(cats, func(i, j int) bool {
		if catTotals[cats[i]] != catTotals[cats[j]] {
			return catTotals[cats[i]] > catTotals[cats[j]]
		}
		return cats[i] < cats[j]
	})
	for _, c := range cats {
		p.Solved.ByCategory = append(p.Solved.ByCategory,
			solvedBucket{Key: c, Solved: catSolved[c], Total: catTotals[c]})
	}

	// Attempting: problems opened in the workspace or submitted, not yet solved.
	rows, err = pool.Query(ctx, `
		SELECT challenge_id FROM candidate_sessions WHERE user_id = $1
		UNION
		SELECT pr.slug FROM submissions s JOIN problems pr ON pr.id = s.problem_id WHERE s.user_id = $1`,
		u.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var slug string
		if err := rows.Scan(&slug); err != nil {
			return err
		}
		if !solvedSlugs[slug] {
			p.Solved.Attempting++
		}
	}
	return rows.Err()
}

func loadCalendar(ctx context.Context, pool *pgxpool.Pool, u *auth.User, now time.Time, p *profileResponse) error {
	today := now.Truncate(24 * time.Hour)
	start := today.AddDate(-1, 0, 1)
	rows, err := pool.Query(ctx, `
		SELECT (submitted_at AT TIME ZONE 'UTC')::date, COUNT(*)
		FROM submissions
		WHERE user_id = $1 AND submitted_at >= $2
		GROUP BY 1 ORDER BY 1`, u.ID, start)
	if err != nil {
		return err
	}
	defer rows.Close()
	counts := map[string]int{}
	p.Calendar.Days = []calendarDay{}
	for rows.Next() {
		var d time.Time
		var n int
		if err := rows.Scan(&d, &n); err != nil {
			return err
		}
		key := d.Format("2006-01-02")
		counts[key] = n
		p.Calendar.Days = append(p.Calendar.Days, calendarDay{Date: key, Count: n})
		p.Calendar.TotalSubmissions += n
	}
	if err := rows.Err(); err != nil {
		return err
	}
	p.Calendar.ActiveDays = len(counts)
	p.Calendar.CurrentStreak, p.Calendar.MaxStreak = streaks(counts, start, today)
	return nil
}

// streaks walks every day in [start, today] and returns the run of active
// days ending today (or yesterday, so an unfinished today doesn't reset it)
// and the longest run in the window.
func streaks(active map[string]int, start, today time.Time) (current, longest int) {
	run := 0
	for d := start; !d.After(today); d = d.AddDate(0, 0, 1) {
		if active[d.Format("2006-01-02")] > 0 {
			run++
			if run > longest {
				longest = run
			}
		} else {
			run = 0
		}
	}
	current = run
	if current == 0 {
		// Today has no activity yet; count the streak that ended yesterday.
		for d := today.AddDate(0, 0, -1); !d.Before(start) && active[d.Format("2006-01-02")] > 0; d = d.AddDate(0, 0, -1) {
			current++
		}
	}
	return current, longest
}

func loadDimensions(ctx context.Context, pool *pgxpool.Pool, u *auth.User, p *profileResponse) error {
	rows, err := pool.Query(ctx, `
		SELECT d.key, AVG((d.value->>'score')::float8), COUNT(*)
		FROM submissions s,
		     jsonb_each(COALESCE(s.scores->'dimension_scores', '{}'::jsonb)) AS d
		WHERE s.user_id = $1 AND s.status = 'graded'
		  AND jsonb_typeof(d.value->'score') = 'number'
		GROUP BY d.key`, u.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	got := map[string]dimensionAverage{}
	for rows.Next() {
		var da dimensionAverage
		var avg float64
		if err := rows.Scan(&da.Dimension, &avg, &da.Samples); err != nil {
			return err
		}
		da.Average = &avg
		got[da.Dimension] = da
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, dim := range profileDimensions {
		da, ok := got[dim]
		if !ok {
			da = dimensionAverage{Dimension: dim}
		}
		p.Dimensions = append(p.Dimensions, da)
	}
	return pool.QueryRow(ctx, `
		SELECT MAX(final_score)::float8 FROM submissions WHERE user_id = $1 AND status = 'graded'`,
		u.ID).Scan(&p.BestScore)
}

func loadCommunity(ctx context.Context, pool *pgxpool.Pool, u *auth.User, now time.Time, p *profileResponse) error {
	weekAgo := now.AddDate(0, 0, -7)
	c := &p.Community
	// Anonymous posts/comments never count toward a public identity.
	return pool.QueryRow(ctx, `
		SELECT
		  (SELECT COUNT(*) FROM forum_posts WHERE user_id = $1 AND deleted_at IS NULL AND NOT is_anonymous),
		  (SELECT COUNT(*) FROM forum_posts WHERE user_id = $1 AND deleted_at IS NULL AND NOT is_anonymous AND created_at >= $2),
		  (SELECT COUNT(*) FROM forum_comments WHERE user_id = $1 AND deleted_at IS NULL AND NOT is_anonymous)
		  + (SELECT COUNT(*) FROM comments WHERE user_id = $1 AND deleted_at IS NULL),
		  (SELECT COUNT(*) FROM forum_comments WHERE user_id = $1 AND deleted_at IS NULL AND NOT is_anonymous AND created_at >= $2)
		  + (SELECT COUNT(*) FROM comments WHERE user_id = $1 AND deleted_at IS NULL AND created_at >= $2),
		  (SELECT COALESCE(SUM(upvotes), 0) FROM forum_posts WHERE user_id = $1 AND deleted_at IS NULL AND NOT is_anonymous)
		  + (SELECT COALESCE(SUM(upvotes), 0) FROM forum_comments WHERE user_id = $1 AND deleted_at IS NULL AND NOT is_anonymous)
		  + (SELECT COALESCE(SUM(upvotes), 0) FROM comments WHERE user_id = $1 AND deleted_at IS NULL),
		  (SELECT COUNT(*) FROM forum_post_votes v JOIN forum_posts fp ON fp.id = v.post_id
		     WHERE fp.user_id = $1 AND fp.deleted_at IS NULL AND NOT fp.is_anonymous AND v.value = 1 AND v.voted_at >= $2)
		  + (SELECT COUNT(*) FROM forum_comment_votes v JOIN forum_comments fc ON fc.id = v.comment_id
		     WHERE fc.user_id = $1 AND fc.deleted_at IS NULL AND NOT fc.is_anonymous AND v.value = 1 AND v.voted_at >= $2)`,
		u.ID, weekAgo,
	).Scan(&c.Posts.Total, &c.Posts.LastWeek, &c.Comments.Total, &c.Comments.LastWeek, &c.Upvotes.Total, &c.Upvotes.LastWeek)
}

func loadRecent(ctx context.Context, pool *pgxpool.Pool, u *auth.User, p *profileResponse) error {
	rows, err := pool.Query(ctx, `
		SELECT s.id::text, pr.slug, pr.title, pr.difficulty, s.status, s.final_score::float8, s.submitted_at
		FROM submissions s JOIN problems pr ON pr.id = s.problem_id
		WHERE s.user_id = $1
		ORDER BY s.submitted_at DESC LIMIT 15`, u.ID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var s profileSubmission
		if err := rows.Scan(&s.ID, &s.ProblemSlug, &s.ProblemTitle, &s.Difficulty, &s.Status, &s.FinalScore, &s.SubmittedAt); err != nil {
			rows.Close()
			return err
		}
		p.RecentSubmissions = append(p.RecentSubmissions, s)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	rows, err = pool.Query(ctx, `
		SELECT id::text, title, section, upvotes, comment_count, created_at
		FROM forum_posts
		WHERE user_id = $1 AND deleted_at IS NULL AND NOT is_anonymous
		ORDER BY created_at DESC LIMIT 10`, u.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var fp profilePost
		if err := rows.Scan(&fp.ID, &fp.Title, &fp.Section, &fp.Upvotes, &fp.CommentCount, &fp.CreatedAt); err != nil {
			return err
		}
		p.RecentPosts = append(p.RecentPosts, fp)
	}
	return rows.Err()
}
