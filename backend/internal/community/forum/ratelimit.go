package forum

import (
	"fmt"
	"net/http"

	"codritium/backend/internal/auth"
	"codritium/backend/internal/platform/httpx"
)

// forumLimit is one per-user write limit, counted from the rows the user
// already wrote in the window. Queries are fixed strings; $1 is the user.
type forumLimit struct {
	max    int
	window string // shown to the client, e.g. "1 hour"
	count  string
}

var (
	postLimit = forumLimit{forumPostsPerHour, "1 hour",
		`SELECT count(*) FROM forum_posts WHERE user_id = $1 AND created_at > now() - interval '1 hour'`}
	commentLimit = forumLimit{forumCommentsPerHour, "1 hour",
		`SELECT count(*) FROM forum_comments WHERE user_id = $1 AND created_at > now() - interval '1 hour'`}
	voteLimit = forumLimit{forumVotesPerMinute, "1 minute",
		`SELECT (SELECT count(*) FROM forum_post_votes WHERE user_id = $1 AND voted_at > now() - interval '1 minute')
		      + (SELECT count(*) FROM forum_comment_votes WHERE user_id = $1 AND voted_at > now() - interval '1 minute')`}
)

// allow reports whether u may make another write under l, writing a 429
// when not. Admins are never limited. The check and the write aren't atomic,
// so a burst of parallel requests can overshoot slightly; that's fine for
// spam control.
func (h Handler) allow(w http.ResponseWriter, r *http.Request, u *auth.User, l forumLimit) bool {
	if u.Role == "admin" {
		return true
	}
	var n int
	if err := h.Pool.QueryRow(r.Context(), l.count, u.ID).Scan(&n); err != nil {
		httpx.Internal(w, r, err)
		return false
	}
	if n >= l.max {
		httpx.ErrorWith(w, http.StatusTooManyRequests, "rate_limited",
			fmt.Sprintf("You're going a bit fast. Try again in a little while (limit %d per %s).", l.max, l.window),
			map[string]any{"limit": l.max, "window": l.window})
		return false
	}
	return true
}
