// Package notifications is the in-app inbox: what other people did that a
// user should hear about (replies, mentions, follows, vote milestones).
// Features decide who to notify and write rows with Insert; this package
// owns reading and marking them read.
package notifications

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"codritium/backend/internal/auth"
	"codritium/backend/internal/platform/httpx"
)

// Kinds, matching the notifications.kind CHECK constraint.
const (
	PostReply     = "post_reply"
	CommentReply  = "comment_reply"
	Mention       = "mention"
	PostComment   = "post_comment"
	PostMilestone = "post_milestone"
)

const (
	pageDefault = 20
	pageMax     = 50
	offsetMax   = 1000
	excerptLen  = 140
)

// New is one notification to write.
type New struct {
	UserID         uuid.UUID
	Kind           string
	ActorID        *uuid.UUID
	ActorAnonymous bool
	PostID         uuid.UUID
	CommentID      *uuid.UUID
}

// Execer is satisfied by a pool or a transaction, so notifications can be
// written atomically with the comment that caused them.
type Execer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// Insert writes ns in one statement. Nothing is written for an empty slice.
func Insert(ctx context.Context, db Execer, ns []New) error {
	if len(ns) == 0 {
		return nil
	}
	users := make([]uuid.UUID, len(ns))
	kinds := make([]string, len(ns))
	actors := make([]*uuid.UUID, len(ns))
	anon := make([]bool, len(ns))
	posts := make([]uuid.UUID, len(ns))
	comments := make([]*uuid.UUID, len(ns))
	for i, n := range ns {
		users[i], kinds[i], actors[i], anon[i], posts[i], comments[i] =
			n.UserID, n.Kind, n.ActorID, n.ActorAnonymous, n.PostID, n.CommentID
	}
	_, err := db.Exec(ctx, `
		INSERT INTO notifications (user_id, kind, actor_id, actor_anonymous, post_id, comment_id)
		SELECT * FROM unnest($1::uuid[], $2::text[], $3::uuid[], $4::bool[], $5::uuid[], $6::uuid[])`,
		users, kinds, actors, anon, posts, comments)
	return err
}

// InsertMilestone records that a post reached milestone upvotes, once per
// post and milestone however often the score crosses it.
func InsertMilestone(ctx context.Context, db Execer, userID, postID uuid.UUID, milestone int) error {
	_, err := db.Exec(ctx, `
		INSERT INTO notifications (user_id, kind, post_id, milestone)
		VALUES ($1, 'post_milestone', $2, $3)
		ON CONFLICT (user_id, post_id, milestone) WHERE kind = 'post_milestone' DO NOTHING`,
		userID, postID, milestone)
	return err
}

type actor struct {
	ID          uuid.UUID `json:"id"`
	Handle      string    `json:"handle"`
	DisplayName string    `json:"display_name"`
	AvatarURL   string    `json:"avatar_url"`
	AvatarColor string    `json:"avatar_color"`
}

type notification struct {
	ID        uuid.UUID  `json:"id"`
	Kind      string     `json:"kind"`
	Actor     *actor     `json:"actor"` // nil for system events and anonymous actors
	PostID    uuid.UUID  `json:"post_id"`
	PostTitle string     `json:"post_title"`
	CommentID *uuid.UUID `json:"comment_id"`
	Excerpt   string     `json:"excerpt"`
	Milestone *int       `json:"milestone"`
	Read      bool       `json:"read"`
	CreatedAt time.Time  `json:"created_at"`
}

type Handler struct {
	Pool *pgxpool.Pool
}

func (h Handler) Routes(rt *httpx.Router) {
	rt.Handle("GET /api/notifications", h.list)
	rt.Handle("GET /api/notifications/unread-count", h.unreadCount)
	rt.Handle("POST /api/notifications/read", h.markRead)
}

// visible hides notifications whose post or comment has since been deleted.
const visible = `
	n.user_id = $1
	AND p.deleted_at IS NULL
	AND (n.comment_id IS NULL OR c.deleted_at IS NULL)`

const from = `
	FROM notifications n
	JOIN forum_posts p ON p.id = n.post_id
	LEFT JOIN forum_comments c ON c.id = n.comment_id
	LEFT JOIN users a ON a.id = n.actor_id`

// List returns the newest notifications first (?unread=1 for unread only).
func (h Handler) list(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.Require(w, r)
	if !ok {
		return
	}
	page := httpx.ParsePage(r, pageDefault, pageMax, offsetMax)
	where := visible
	if r.URL.Query().Get("unread") == "1" {
		where += " AND n.read_at IS NULL"
	}
	rows, err := h.Pool.Query(r.Context(), `
		SELECT n.id, n.kind, n.actor_anonymous, n.post_id, p.title, n.comment_id,
		       COALESCE(left(regexp_replace(c.body, '\s+', ' ', 'g'), $4), ''),
		       n.milestone, n.read_at IS NOT NULL, n.created_at,
		       a.id, COALESCE(a.handle, ''), COALESCE(a.display_name, ''),
		       COALESCE(a.avatar_url, ''), COALESCE(a.avatar_color, '')`+from+`
		WHERE `+where+`
		ORDER BY n.created_at DESC, n.id DESC
		LIMIT $2 OFFSET $3`, u.ID, page.Limit+1, page.Offset, excerptLen)
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	defer rows.Close()
	items := make([]notification, 0, page.Limit)
	for rows.Next() {
		var (
			n       notification
			anon    bool
			actorID *uuid.UUID
			a       actor
		)
		if err := rows.Scan(&n.ID, &n.Kind, &anon, &n.PostID, &n.PostTitle, &n.CommentID,
			&n.Excerpt, &n.Milestone, &n.Read, &n.CreatedAt,
			&actorID, &a.Handle, &a.DisplayName, &a.AvatarURL, &a.AvatarColor); err != nil {
			httpx.Internal(w, r, err)
			return
		}
		if actorID != nil && !anon {
			a.ID = *actorID
			n.Actor = &a
		}
		items = append(items, n)
	}
	if err := rows.Err(); err != nil {
		httpx.Internal(w, r, err)
		return
	}
	hasMore := len(items) > page.Limit
	if hasMore {
		items = items[:page.Limit]
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"notifications": items, "has_more": hasMore})
}

func (h Handler) unreadCount(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.Require(w, r)
	if !ok {
		return
	}
	var n int
	if err := h.Pool.QueryRow(r.Context(),
		`SELECT count(*)`+from+` WHERE `+visible+` AND n.read_at IS NULL`, u.ID).Scan(&n); err != nil {
		httpx.Internal(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"count": n})
}

// MarkRead marks the given notifications read, or all of them with
// {"all": true}. Ids that aren't the caller's are ignored.
func (h Handler) markRead(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.Require(w, r)
	if !ok {
		return
	}
	var req struct {
		IDs []uuid.UUID `json:"ids"`
		All bool        `json:"all"`
	}
	if !httpx.Decode(w, r, &req) {
		return
	}
	var err error
	switch {
	case req.All:
		_, err = h.Pool.Exec(r.Context(),
			`UPDATE notifications SET read_at = now() WHERE user_id = $1 AND read_at IS NULL`, u.ID)
	case len(req.IDs) > 0 && len(req.IDs) <= pageMax:
		_, err = h.Pool.Exec(r.Context(),
			`UPDATE notifications SET read_at = now() WHERE user_id = $1 AND id = ANY($2) AND read_at IS NULL`,
			u.ID, req.IDs)
	default:
		httpx.BadRequest(w, "invalid_ids", "Send 1-50 ids or all=true.")
		return
	}
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
