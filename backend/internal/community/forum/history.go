package forum

import (
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"

	"codritium/backend/internal/auth"
	"codritium/backend/internal/platform/httpx"
)

const revisionsMax = 50

type forumRevision struct {
	Title      string    `json:"title"`
	BodyMD     string    `json:"body_md"`
	Tags       []string  `json:"tags"`
	WrittenAt  time.Time `json:"written_at"`
	ReplacedAt time.Time `json:"replaced_at"`
}

// ListForumPostRevisions returns a post's earlier versions, newest first.
// Only the author and moderators can see them; everyone else gets 404 as
// if there were no history.
func (h Handler) listForumPostRevisions(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.PathUUID(w, r, "id")
	if !ok {
		return
	}
	u, ok := auth.Require(w, r)
	if !ok {
		return
	}
	var allowed bool
	err := h.Pool.QueryRow(r.Context(),
		`SELECT user_id = $2 OR $3 FROM forum_posts WHERE id = $1 AND deleted_at IS NULL`,
		id, u.ID, u.Role == "admin").Scan(&allowed)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && !allowed) {
		forumError(w, http.StatusNotFound, "not_found")
		return
	}
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	rows, err := h.Pool.Query(r.Context(), `
		SELECT title, body_md, tags, written_at, replaced_at FROM forum_post_revisions
		WHERE post_id = $1 ORDER BY replaced_at DESC LIMIT $2`, id, revisionsMax)
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	revs, err := pgx.CollectRows(rows, pgx.RowToStructByPos[forumRevision])
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"revisions": revs})
}

// CountNewForumComments says how many comments other people have written on
// a post since ?since (the as_of of the page's last comment load), so the
// page can offer to show them. Public; signed-in viewers don't count their
// own comments, which they already see.
func (h Handler) countNewForumComments(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.PathUUID(w, r, "id")
	if !ok {
		return
	}
	since, err := time.Parse(time.RFC3339Nano, r.URL.Query().Get("since"))
	if err != nil {
		httpx.BadRequest(w, "invalid_since", "since must be an RFC 3339 time.")
		return
	}
	var n int
	if err := h.Pool.QueryRow(r.Context(), `
		SELECT count(*) FROM forum_comments
		WHERE post_id = $1 AND deleted_at IS NULL AND created_at > $2 AND user_id <> $3`,
		id, since, viewerID(auth.FromContext(r.Context()))).Scan(&n); err != nil {
		httpx.Internal(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]int{"count": n})
}
