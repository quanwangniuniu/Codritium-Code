package forum

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"codritium/backend/internal/community/votes"
	"codritium/backend/internal/platform/httpx"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"codritium/backend/internal/auth"
)

const forumCommentColumns = `
	c.id, c.post_id, c.parent_id, c.user_id, c.body, c.is_anonymous,
	c.upvotes, c.downvotes, c.created_at,
	CASE WHEN c.updated_at > c.created_at THEN c.updated_at END,
	c.deleted_at IS NOT NULL,
	u.handle, u.display_name, COALESCE(u.avatar_url,''), COALESCE(u.avatar_color,''),
	COALESCE(u.role,''), COALESCE(v.value, 0),` + reputationExpr

const forumCommentFrom = `
	FROM forum_comments c
	JOIN users u ON u.id = c.user_id
	LEFT JOIN forum_comment_votes v ON v.comment_id = c.id AND v.user_id = $1`

func scanForumComment(row pgx.Row, viewer *auth.User, postAuthor uuid.UUID) (forumComment, error) {
	var (
		c        forumComment
		authorID uuid.UUID
		a        forumAuthor
		role     string
	)
	if err := row.Scan(&c.ID, &c.PostID, &c.ParentID, &authorID, &c.Body, &c.IsAnonymous,
		&c.Upvotes, &c.Downvotes, &c.CreatedAt, &c.EditedAt, &c.IsDeleted,
		&a.Handle, &a.DisplayName, &a.AvatarURL, &a.AvatarColor, &role, &c.MyVote, &a.Reputation); err != nil {
		return forumComment{}, err
	}
	c.Score = c.Upvotes - c.Downvotes
	if c.IsDeleted {
		// A placeholder keeps its place in the thread but says nothing about
		// what was written or by whom.
		c.Body, c.IsAnonymous, c.EditedAt, c.MyVote = "", false, nil, 0
		return c, nil
	}
	c.IsMine = viewer != nil && viewer.ID == authorID
	c.IsOP = authorID == postAuthor
	if canSeeAuthor(viewer, authorID, c.IsAnonymous) {
		a.ID = authorID
		a.Verified = role == "admin"
		c.Author = &a
	}
	return c, nil
}

type forumPostMeta struct {
	authorID uuid.UUID
	section  string
	locked   bool
}

func loadForumPostMeta(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, id uuid.UUID) (forumPostMeta, error) {
	var m forumPostMeta
	err := q.QueryRow(ctx,
		`SELECT user_id, section, is_locked FROM forum_posts WHERE id = $1 AND deleted_at IS NULL`, id,
	).Scan(&m.authorID, &m.section, &m.locked)
	return m, err
}

// ListForumComments returns a page of top-level comments (sort=best|newest)
// with all of their replies, oldest reply first. A deleted top-level comment
// that still has replies stays in the list as a placeholder.
func (h Handler) listForumComments(w http.ResponseWriter, r *http.Request) {
	postID, ok := httpx.PathUUID(w, r, "id")
	if !ok {
		return
	}
	viewer := auth.FromContext(r.Context())
	meta, err := loadForumPostMeta(r.Context(), h.Pool, postID)
	if errors.Is(err, pgx.ErrNoRows) {
		forumError(w, http.StatusNotFound, "not_found")
		return
	}
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	// as_of (read before the comments, from the clock that stamps them)
	// lets the page ask later how many comments arrived since this load.
	var asOf time.Time
	if err := h.Pool.QueryRow(r.Context(), `SELECT now()`).Scan(&asOf); err != nil {
		httpx.Internal(w, r, err)
		return
	}
	page := httpx.ParsePage(r, forumPageDefault, forumPageMax, forumOffsetMax)
	limit, offset := page.Limit, page.Offset
	order := "(c.upvotes - c.downvotes) DESC, c.created_at DESC, c.id DESC"
	switch r.URL.Query().Get("sort") {
	case "", "best":
	case "newest":
		order = "c.created_at DESC, c.id DESC"
	default:
		forumError(w, http.StatusBadRequest, "invalid_sort")
		return
	}

	rows, err := h.Pool.Query(r.Context(), `SELECT `+forumCommentColumns+forumCommentFrom+`
		WHERE c.post_id = $2 AND c.parent_id IS NULL
		  AND (c.deleted_at IS NULL OR EXISTS (
		    SELECT 1 FROM forum_comments r WHERE r.parent_id = c.id AND r.deleted_at IS NULL))
		ORDER BY `+order+`
		LIMIT $3 OFFSET $4`, viewerID(viewer), postID, limit+1, offset)
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	top := make([]forumComment, 0, limit)
	for rows.Next() {
		c, err := scanForumComment(rows, viewer, meta.authorID)
		if err != nil {
			rows.Close()
			httpx.Internal(w, r, err)
			return
		}
		top = append(top, c)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		httpx.Internal(w, r, err)
		return
	}
	hasMore := len(top) > limit
	if hasMore {
		top = top[:limit]
	}

	if len(top) > 0 {
		ids := make([]uuid.UUID, len(top))
		index := make(map[uuid.UUID]int, len(top))
		for i, c := range top {
			ids[i] = c.ID
			index[c.ID] = i
		}
		rows, err := h.Pool.Query(r.Context(), `SELECT `+forumCommentColumns+forumCommentFrom+`
			WHERE c.parent_id = ANY($2) AND c.deleted_at IS NULL
			ORDER BY c.created_at, c.id`, viewerID(viewer), ids)
		if err != nil {
			httpx.Internal(w, r, err)
			return
		}
		defer rows.Close()
		for rows.Next() {
			c, err := scanForumComment(rows, viewer, meta.authorID)
			if err != nil {
				httpx.Internal(w, r, err)
				return
			}
			i := index[*c.ParentID]
			top[i].Replies = append(top[i].Replies, c)
		}
	}
	httpx.JSON(w, http.StatusOK, map[string]any{
		"comments": top, "has_more": hasMore, "as_of": asOf.UTC().Format(time.RFC3339Nano),
	})
}

type forumCommentRequest struct {
	Body        string     `json:"body"`
	ParentID    *uuid.UUID `json:"parent_id"`
	IsAnonymous bool       `json:"is_anonymous"`
}

// CreateForumComment adds a comment or a reply. Replies attach to top-level
// comments only, so threads stay one level deep.
func (h Handler) createForumComment(w http.ResponseWriter, r *http.Request) {
	postID, ok := httpx.PathUUID(w, r, "id")
	if !ok {
		return
	}
	u, ok := auth.Require(w, r)
	if !ok {
		return
	}
	var req forumCommentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		forumError(w, http.StatusBadRequest, "bad_json")
		return
	}
	body, ok := validCommentBody(w, req.Body)
	if !ok || !h.allow(w, r, u, commentLimit, body) {
		return
	}

	tx, err := h.Pool.BeginTx(r.Context(), pgx.TxOptions{})
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	defer tx.Rollback(r.Context())

	meta, err := loadForumPostMeta(r.Context(), tx, postID)
	if errors.Is(err, pgx.ErrNoRows) {
		forumError(w, http.StatusNotFound, "not_found")
		return
	}
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	// Locked threads take no new comments, except a moderator's.
	if meta.locked && u.Role != "admin" {
		forumError(w, http.StatusForbidden, "locked")
		return
	}
	if req.IsAnonymous && !forumAnonSections[meta.section] {
		forumError(w, http.StatusBadRequest, "anonymous_not_allowed")
		return
	}
	var parentAuthor *uuid.UUID
	if req.ParentID != nil {
		var parentPost, author uuid.UUID
		var grandparent *uuid.UUID
		err := tx.QueryRow(r.Context(),
			`SELECT post_id, parent_id, user_id FROM forum_comments WHERE id = $1 AND deleted_at IS NULL`,
			*req.ParentID).Scan(&parentPost, &grandparent, &author)
		parentAuthor = &author
		if err != nil || parentPost != postID || grandparent != nil {
			forumError(w, http.StatusBadRequest, "invalid_parent")
			return
		}
	}

	var id uuid.UUID
	if err := tx.QueryRow(r.Context(), `
		INSERT INTO forum_comments (post_id, user_id, parent_id, body, is_anonymous)
		VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		postID, u.ID, req.ParentID, body, req.IsAnonymous).Scan(&id); err != nil {
		httpx.Internal(w, r, err)
		return
	}
	if _, err := tx.Exec(r.Context(),
		`UPDATE forum_posts SET comment_count = comment_count + 1 WHERE id = $1`, postID); err != nil {
		httpx.Internal(w, r, err)
		return
	}
	if err := notifyNewComment(r.Context(), tx, commentEvent{
		postID: postID, postAuthor: meta.authorID, commentID: id, actor: u.ID,
		parentAuthor: parentAuthor, body: body, anonymous: req.IsAnonymous,
	}); err != nil {
		httpx.Internal(w, r, err)
		return
	}
	c, err := scanForumComment(tx.QueryRow(r.Context(),
		`SELECT `+forumCommentColumns+forumCommentFrom+` WHERE c.id = $2`, u.ID, id), u, meta.authorID)
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		httpx.Internal(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, c)
}

func (h Handler) voteForumComment(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.PathUUID(w, r, "id")
	if !ok {
		return
	}
	u, ok := auth.Require(w, r)
	if !ok || !h.allow(w, r, u, voteLimit, "") {
		return
	}
	votes.Handle(w, r, h.Pool, votes.ForumComment, id, u.ID)
}

func validCommentBody(w http.ResponseWriter, raw string) (string, bool) {
	body := strings.TrimSpace(raw)
	if body == "" || utf8.RuneCountInString(body) > forumCommentMax {
		forumError(w, http.StatusBadRequest, "invalid_body")
		return "", false
	}
	return body, true
}

// UpdateForumComment replaces a comment's text. Only the author may edit
// (admins moderate by deleting); anyone else gets the same 404 as a missing
// comment. The edit shows as edited_at.
func (h Handler) updateForumComment(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.PathUUID(w, r, "id")
	if !ok {
		return
	}
	u, ok := auth.Require(w, r)
	if !ok {
		return
	}
	var req struct {
		Body string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		forumError(w, http.StatusBadRequest, "bad_json")
		return
	}
	body, ok := validCommentBody(w, req.Body)
	if !ok || !h.allow(w, r, u, editLimit, "") {
		return
	}
	var postAuthor uuid.UUID
	err := h.Pool.QueryRow(r.Context(), `
		UPDATE forum_comments c SET body = $3, updated_at = now()
		FROM forum_posts p
		WHERE c.id = $1 AND c.user_id = $2 AND c.deleted_at IS NULL
		  AND p.id = c.post_id AND p.deleted_at IS NULL AND NOT p.is_locked
		RETURNING p.user_id`, id, u.ID, body).Scan(&postAuthor)
	if errors.Is(err, pgx.ErrNoRows) {
		forumError(w, http.StatusNotFound, "not_found")
		return
	}
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	c, err := scanForumComment(h.Pool.QueryRow(r.Context(),
		`SELECT `+forumCommentColumns+forumCommentFrom+` WHERE c.id = $2`, u.ID, id), u, postAuthor)
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, c)
}

// DeleteForumComment soft-deletes a comment (author or admin). Replies stay:
// a deleted top-level comment with replies lists as a "[deleted]"
// placeholder, so the conversation under it isn't lost.
func (h Handler) deleteForumComment(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.PathUUID(w, r, "id")
	if !ok {
		return
	}
	u, ok := auth.Require(w, r)
	if !ok {
		return
	}
	err := softDeleteComment(r.Context(), h.Pool, id, u.ID, u.Role == "admin")
	if errors.Is(err, pgx.ErrNoRows) {
		forumError(w, http.StatusNotFound, "not_found")
		return
	}
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// softDeleteComment deletes comment id if by wrote it or byAdmin, keeping
// the post's comment count in step. pgx.ErrNoRows means there was nothing
// (left) to delete.
func softDeleteComment(ctx context.Context, pool *pgxpool.Pool, id, by uuid.UUID, byAdmin bool) error {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var postID uuid.UUID
	if err := tx.QueryRow(ctx, `
		UPDATE forum_comments SET deleted_at = now()
		WHERE id = $1 AND deleted_at IS NULL AND (user_id = $2 OR $3)
		RETURNING post_id`, id, by, byAdmin).Scan(&postID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`UPDATE forum_posts SET comment_count = GREATEST(comment_count - 1, 0) WHERE id = $1`, postID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
