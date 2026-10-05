package forum

import (
	"net/http"

	"codritium/backend/internal/auth"
	"codritium/backend/internal/platform/httpx"

	"github.com/google/uuid"
)

// visitorCookie identifies a signed-out reader so their views are counted
// once a day rather than on every page load. It holds a random UUID and
// nothing else.
const visitorCookie = "codritium_vid"

const visitorCookieMaxAge = 365 * 24 * 60 * 60

// RecordForumView counts a read of a post, sent by the post page once it is
// on screen. Signed-in users are keyed by account and signed-out readers by
// a visitor cookie (set here on first view); either way the same reader
// counts at most once per post per day. Responds with the current count.
func (h Handler) recordForumView(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.PathUUID(w, r, "id")
	if !ok {
		return
	}

	// Each branch upserts the reader's row, refreshing viewed_at only when
	// the last counted view is over a day old; RETURNING yields a row only
	// when something was written, which is exactly when the view counts.
	table, readerCol := "forum_post_views", "user_id"
	var reader uuid.UUID
	if u := auth.FromContext(r.Context()); u != nil {
		reader = u.ID
	} else {
		reader = h.visitorID(w, r)
		table, readerCol = "forum_post_visitor_views", "visitor_id"
	}
	var count int
	err := h.Pool.QueryRow(r.Context(), `
		WITH ins AS (
		  INSERT INTO `+table+` AS v (post_id, `+readerCol+`)
		  SELECT $1, $2 WHERE EXISTS (SELECT 1 FROM forum_posts WHERE id = $1 AND deleted_at IS NULL)
		  ON CONFLICT (post_id, `+readerCol+`) DO UPDATE SET viewed_at = now()
		    WHERE v.viewed_at < now() - interval '1 day'
		  RETURNING post_id
		), bump AS (
		  UPDATE forum_posts SET view_count = view_count + 1
		  WHERE id IN (SELECT post_id FROM ins)
		  RETURNING view_count
		)
		SELECT COALESCE((SELECT view_count FROM bump),
		                (SELECT view_count FROM forum_posts WHERE id = $1 AND deleted_at IS NULL), -1)`,
		id, reader).Scan(&count)
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	if count < 0 {
		forumError(w, http.StatusNotFound, "not_found")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"view_count": count})
}

// visitorID returns the reader's visitor cookie, issuing a new one when it is
// missing or malformed.
func (h Handler) visitorID(w http.ResponseWriter, r *http.Request) uuid.UUID {
	if c, err := r.Cookie(visitorCookie); err == nil {
		if id, err := uuid.Parse(c.Value); err == nil {
			return id
		}
	}
	id := uuid.New()
	http.SetCookie(w, &http.Cookie{
		Name:     visitorCookie,
		Value:    id.String(),
		Path:     "/",
		Domain:   h.Cookie.Domain,
		Secure:   h.Cookie.Secure,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   visitorCookieMaxAge,
	})
	return id
}
