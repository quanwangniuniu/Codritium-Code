// Package comments is per-problem discussion, unlocked once the user has a
// graded submission for the problem.
package comments

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"codritium/backend/internal/community/votes"
	"codritium/backend/internal/platform/httpx"
	"codritium/backend/internal/submissions"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"codritium/backend/internal/auth"
)

// Routes registers the problem-comment API.
func (h Handler) Routes(rt *httpx.Router) {
	rt.Handle("GET /api/problems/{slug}/comments", h.listComments)
	rt.Handle("POST /api/problems/{slug}/comments", h.createComment)
	rt.Handle("POST /api/comments/{id}/vote", h.voteComment)
	rt.Handle("DELETE /api/comments/{id}", h.deleteComment)
}

type Handler struct {
	Pool *pgxpool.Pool
}

type commentItem struct {
	ID            uuid.UUID  `json:"id"`
	ProblemSlug   string     `json:"problem_slug"`
	UserID        uuid.UUID  `json:"user_id"`
	UserHandle    string     `json:"user_handle"`
	UserDisplay   string     `json:"user_display_name"`
	UserAvatarURL string     `json:"user_avatar_url"`
	UserTier      string     `json:"user_tier"`
	Body          string     `json:"body"`
	Upvotes       int        `json:"upvotes"`
	Downvotes     int        `json:"downvotes"`
	ParentID      *uuid.UUID `json:"parent_id,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	MyVote        int        `json:"my_vote"`
}

// ListComments returns a page of comments for a problem, newest first. The
// gate matches the reply gate: callers must have completed (graded) the
// problem unless they are admin. cursor is opaque base64; clients send back
// the next_cursor returned by the previous page.
func (h Handler) listComments(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	if slug == "" {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "Slug required.")
		return
	}
	u := auth.FromContext(r.Context())
	if u == nil {
		httpx.Unauthorized(w)
		return
	}
	if !RequireGraded(w, r, submissions.Store{Pool: h.Pool}, u, slug) {
		return
	}

	limit := 20
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 50 {
			limit = n
		}
	}

	cursorTS, cursorID, cursorOK := decodeCommentsCursor(r.URL.Query().Get("cursor"))

	// Keyset pagination: with a cursor, continue after (created_at, id).
	var after *time.Time
	if cursorOK {
		after = &cursorTS
	}
	rows, err := h.Pool.Query(r.Context(), `
		SELECT c.id, c.problem_slug, c.user_id,
		       u.handle, u.display_name, COALESCE(u.avatar_url,''), u.tier,
		       c.body, c.upvotes, c.downvotes, c.parent_id, c.created_at,
		       COALESCE(v.value, 0)
		FROM comments c
		JOIN users u ON u.id = c.user_id
		LEFT JOIN comment_votes v ON v.comment_id = c.id AND v.user_id = $1
		WHERE c.problem_slug = $2
		  AND c.deleted_at IS NULL
		  AND ($3::timestamptz IS NULL OR (c.created_at, c.id) < ($3, $4))
		ORDER BY c.created_at DESC, c.id DESC
		LIMIT $5`,
		u.ID, slug, after, cursorID, limit+1)
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	defer rows.Close()

	items := make([]commentItem, 0, limit)
	for rows.Next() {
		var it commentItem
		if err := rows.Scan(&it.ID, &it.ProblemSlug, &it.UserID,
			&it.UserHandle, &it.UserDisplay, &it.UserAvatarURL, &it.UserTier,
			&it.Body, &it.Upvotes, &it.Downvotes, &it.ParentID, &it.CreatedAt,
			&it.MyVote); err != nil {
			httpx.Internal(w, r, err)
			return
		}
		items = append(items, it)
	}
	next := ""
	if len(items) > limit {
		tail := items[limit-1]
		next = encodeCommentsCursor(tail.CreatedAt, tail.ID)
		items = items[:limit]
	}
	httpx.JSON(w, http.StatusOK, map[string]any{
		"comments":    items,
		"next_cursor": next,
	})
}

type createCommentRequest struct {
	Body     string     `json:"body"`
	ParentID *uuid.UUID `json:"parent_id,omitempty"`
}

// CreateComment inserts a new comment for the caller. Honors the same graded
// gate as ListComments and rejects empty bodies. parent_id is validated to
// belong to the same problem so a reply can't escape its thread.
func (h Handler) createComment(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	if slug == "" {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "Slug required.")
		return
	}
	u := auth.FromContext(r.Context())
	if u == nil {
		httpx.Unauthorized(w)
		return
	}
	if !RequireGraded(w, r, submissions.Store{Pool: h.Pool}, u, slug) {
		return
	}
	var req createCommentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "Bad json.")
		return
	}
	body := strings.TrimSpace(req.Body)
	if body == "" {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "Body required.")
		return
	}

	if req.ParentID != nil {
		var parentSlug string
		err := h.Pool.QueryRow(r.Context(),
			`SELECT problem_slug FROM comments WHERE id = $1 AND deleted_at IS NULL`,
			*req.ParentID,
		).Scan(&parentSlug)
		if err != nil || parentSlug != slug {
			httpx.Error(w, http.StatusBadRequest, "bad_request", "Parent comment not in this problem.")
			return
		}
	}

	var id uuid.UUID
	var createdAt time.Time
	err := h.Pool.QueryRow(r.Context(), `
		INSERT INTO comments (problem_slug, user_id, body, parent_id)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at`,
		slug, u.ID, body, req.ParentID,
	).Scan(&id, &createdAt)
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]any{
		"id":         id,
		"created_at": createdAt,
	})
}

// VoteComment upserts the caller's vote and atomically updates the cached
// upvote/downvote counters on the comment row. value=0 removes any
// existing vote. Returns the fresh counter pair so the client can update
// its UI without re-fetching.
func (h Handler) voteComment(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.PathUUID(w, r, "id")
	if !ok {
		return
	}
	u, ok := auth.Require(w, r)
	if !ok {
		return
	}
	votes.Handle(w, r, h.Pool, votes.ProblemComment, id, u.ID)
}

// DeleteComment soft-deletes the caller's own comment. Returns 404 either
// when the row doesn't exist or belongs to someone else; the response
// shouldn't reveal which.
func (h Handler) deleteComment(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "Bad id.")
		return
	}
	u := auth.FromContext(r.Context())
	if u == nil {
		httpx.Unauthorized(w)
		return
	}
	tag, err := h.Pool.Exec(r.Context(),
		`UPDATE comments SET deleted_at = now() WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL`,
		id, u.ID)
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	if tag.RowsAffected() == 0 {
		httpx.Error(w, http.StatusNotFound, "not_found", "Not found.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// RequireGraded lets admins and users with a graded submission for the
// problem through. Otherwise it writes 403 must_complete_problem (or a 500)
// and returns false. Replays and note sharing use the same gate.
func RequireGraded(w http.ResponseWriter, r *http.Request, subs submissions.Store, u *auth.User, slug string) bool {
	if u.IsAdmin() {
		return true
	}
	ok, err := subs.HasGraded(r.Context(), u.ID, slug)
	if err != nil {
		httpx.Internal(w, r, err)
		return false
	}
	if !ok {
		httpx.ErrorWith(w, http.StatusForbidden, "must_complete_problem",
			"Submit a graded solution to this problem first.", map[string]any{"challenge_slug": slug})
		return false
	}
	return true
}

const cursorSep = "|"

func encodeCommentsCursor(ts time.Time, id uuid.UUID) string {
	raw := ts.UTC().Format(time.RFC3339Nano) + cursorSep + id.String()
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodeCommentsCursor(cursor string) (time.Time, uuid.UUID, bool) {
	if cursor == "" {
		return time.Time{}, uuid.Nil, false
	}
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return time.Time{}, uuid.Nil, false
	}
	parts := strings.SplitN(string(raw), cursorSep, 2)
	if len(parts) != 2 {
		return time.Time{}, uuid.Nil, false
	}
	ts, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return time.Time{}, uuid.Nil, false
	}
	id, err := uuid.Parse(parts[1])
	if err != nil {
		return time.Time{}, uuid.Nil, false
	}
	return ts, id, true
}
