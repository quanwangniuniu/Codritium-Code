package forum

import (
	"fmt"
	"net/http"
	"regexp"
	"time"

	"codritium/backend/internal/auth"
	"codritium/backend/internal/platform/httpx"
)

// NewAccountRules hold back brand-new accounts, the usual source of spam.
// The zero value applies no restrictions (tests rely on this).
type NewAccountRules struct {
	// Wait is how old an account must be before it can post or comment.
	Wait time.Duration
	// LinkWindow is how long an account counts as new for MaxLinks.
	LinkWindow time.Duration
	// MaxLinks caps links in one post or comment from a new account.
	MaxLinks int
}

// DefaultNewAccountRules is what the server runs with.
var DefaultNewAccountRules = NewAccountRules{Wait: 10 * time.Minute, LinkWindow: 24 * time.Hour, MaxLinks: 2}

// forumLimit is one kind of write and its per-user limit, counted from the
// rows the user already wrote in the window. Queries are fixed strings; $1
// is the user. An empty count means the write isn't rate limited (edits),
// though mutes still apply.
type forumLimit struct {
	max    int
	window string // shown to the client, e.g. "1 hour"
	count  string
	// creates marks new posts and comments, which new-account rules gate.
	creates bool
}

var (
	postLimit = forumLimit{forumPostsPerHour, "1 hour",
		`SELECT count(*) FROM forum_posts WHERE user_id = $1 AND created_at > now() - interval '1 hour'`, true}
	commentLimit = forumLimit{forumCommentsPerHour, "1 hour",
		`SELECT count(*) FROM forum_comments WHERE user_id = $1 AND created_at > now() - interval '1 hour'`, true}
	voteLimit = forumLimit{forumVotesPerMinute, "1 minute",
		`SELECT (SELECT count(*) FROM forum_post_votes WHERE user_id = $1 AND voted_at > now() - interval '1 minute')
		      + (SELECT count(*) FROM forum_comment_votes WHERE user_id = $1 AND voted_at > now() - interval '1 minute')`, false}
	reportLimit = forumLimit{forumReportsPerHour, "1 hour",
		`SELECT count(*) FROM forum_reports WHERE reporter_id = $1 AND created_at > now() - interval '1 hour'`, false}
	editLimit = forumLimit{}
)

var linkPattern = regexp.MustCompile(`(?i)https?://`)

// allow reports whether u may make a write of kind l with the given text
// (empty for votes and reports), writing the refusal when not:
//   - 403 muted while a moderator's mute lasts;
//   - 403 account_too_new / too_many_links under the new-account rules;
//   - 429 rate_limited past the limit.
//
// Admins are never held back. The checks and the write aren't atomic, so a
// burst of parallel requests can overshoot a limit slightly; that's fine for
// spam control.
func (h Handler) allow(w http.ResponseWriter, r *http.Request, u *auth.User, l forumLimit, text string) bool {
	if u.Role == "admin" {
		return true
	}
	var mutedUntil *time.Time
	var accountAge time.Duration
	var ageSeconds float64
	if err := h.Pool.QueryRow(r.Context(), `
		SELECT (SELECT muted_until FROM forum_mutes WHERE user_id = $1 AND muted_until > now()),
		       COALESCE((SELECT extract(epoch FROM now() - created_at) FROM users WHERE id = $1), 0)`,
		u.ID).Scan(&mutedUntil, &ageSeconds); err != nil {
		httpx.Internal(w, r, err)
		return false
	}
	accountAge = time.Duration(ageSeconds * float64(time.Second))
	if mutedUntil != nil {
		httpx.ErrorWith(w, http.StatusForbidden, "muted",
			"A moderator has paused your posting until "+mutedUntil.UTC().Format("Jan 2, 15:04 UTC")+".",
			map[string]any{"muted_until": mutedUntil})
		return false
	}
	rules := h.NewAccount
	if l.creates && rules.Wait > 0 && accountAge < rules.Wait {
		httpx.ErrorWith(w, http.StatusForbidden, "account_too_new",
			fmt.Sprintf("New accounts can post after %d minutes. Thanks for your patience.", int(rules.Wait.Minutes())),
			map[string]any{"wait_seconds": int((rules.Wait - accountAge).Seconds())})
		return false
	}
	if l.creates && rules.MaxLinks > 0 && accountAge < rules.LinkWindow &&
		len(linkPattern.FindAllStringIndex(text, -1)) > rules.MaxLinks {
		httpx.ErrorWith(w, http.StatusForbidden, "too_many_links",
			fmt.Sprintf("New accounts can include up to %d links per post.", rules.MaxLinks),
			map[string]any{"max_links": rules.MaxLinks})
		return false
	}
	if l.count == "" {
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
