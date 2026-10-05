package forum

import (
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"codritium/backend/internal/auth"
	"codritium/backend/internal/platform/httpx"
)

const (
	reportDetailsMax = 500
	muteDaysMax      = 365
)

var reportReasons = map[string]bool{"spam": true, "abuse": true, "off_topic": true, "other": true}

// requireAdmin is auth.Require plus the admin check.
func requireAdmin(w http.ResponseWriter, r *http.Request) (*auth.User, bool) {
	u, ok := auth.Require(w, r)
	if !ok {
		return nil, false
	}
	if u.Role != "admin" {
		forumError(w, http.StatusForbidden, "admin_only")
		return nil, false
	}
	return u, true
}

// reportTarget names a post, or a comment on it.
type reportTarget struct {
	PostID    uuid.UUID  `json:"post_id"`
	CommentID *uuid.UUID `json:"comment_id"`
}

// CreateForumReport flags a post or comment for moderators. A reader has at
// most one open report per target (409 already_reported).
func (h Handler) createForumReport(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.Require(w, r)
	if !ok {
		return
	}
	var req struct {
		reportTarget
		Reason  string `json:"reason"`
		Details string `json:"details"`
	}
	if !httpx.Decode(w, r, &req) {
		return
	}
	req.Details = strings.TrimSpace(req.Details)
	if !reportReasons[req.Reason] {
		httpx.BadRequest(w, "invalid_reason", "Pick a reason: spam, abuse, off_topic, or other.")
		return
	}
	if utf8.RuneCountInString(req.Details) > reportDetailsMax {
		httpx.BadRequest(w, "invalid_details", "Keep details under 500 characters.")
		return
	}
	if !h.allow(w, r, u, reportLimit, "") {
		return
	}
	// Inserts nothing (no row back) when the target is missing or deleted.
	var id uuid.UUID
	err := h.Pool.QueryRow(r.Context(), `
		INSERT INTO forum_reports (reporter_id, post_id, comment_id, reason, details)
		SELECT $1, p.id, $3, $4, $5 FROM forum_posts p
		WHERE p.id = $2 AND p.deleted_at IS NULL
		  AND ($3::uuid IS NULL OR EXISTS (
		    SELECT 1 FROM forum_comments c WHERE c.id = $3 AND c.post_id = p.id AND c.deleted_at IS NULL))
		RETURNING id`, u.ID, req.PostID, req.CommentID, req.Reason, req.Details).Scan(&id)
	var pgErr *pgconn.PgError
	switch {
	case errors.As(err, &pgErr) && pgErr.Code == "23505":
		httpx.Error(w, http.StatusConflict, "already_reported", "You've already reported this. Moderators will take a look.")
	case errors.Is(err, pgx.ErrNoRows):
		forumError(w, http.StatusNotFound, "not_found")
	case err != nil:
		httpx.Internal(w, r, err)
	default:
		w.WriteHeader(http.StatusCreated)
	}
}

type reportedAuthor struct {
	ID          uuid.UUID  `json:"id"`
	Handle      string     `json:"handle"`
	DisplayName string     `json:"display_name"`
	MutedUntil  *time.Time `json:"muted_until"`
}

type reportedItem struct {
	reportTarget
	PostTitle   string         `json:"post_title"`
	Excerpt     string         `json:"excerpt"`
	IsAnonymous bool           `json:"is_anonymous"`
	Author      reportedAuthor `json:"author"`
	Reports     int            `json:"reports"`
	Reasons     []string       `json:"reasons"`
	Details     []string       `json:"details"`
	FirstAt     time.Time      `json:"first_reported_at"`
	LastAt      time.Time      `json:"last_reported_at"`
	// Removed is true when the content is already gone (deleted by its
	// author or another moderator) while reports are still open.
	Removed bool `json:"removed"`
}

// ListForumReports is the moderation queue: one row per reported post or
// comment with open reports, most-reported first. Admins see authors even
// of anonymous content.
func (h Handler) listForumReports(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdmin(w, r); !ok {
		return
	}
	page := httpx.ParsePage(r, forumPageDefault, forumPageMax, forumOffsetMax)
	rows, err := h.Pool.Query(r.Context(), `
		SELECT rp.post_id, rp.comment_id, p.title,
		       left(regexp_replace(COALESCE(c.body, p.body_md), '\s+', ' ', 'g'), 300),
		       COALESCE(c.is_anonymous, p.is_anonymous),
		       a.id, a.handle, a.display_name,
		       (SELECT muted_until FROM forum_mutes m WHERE m.user_id = a.id AND m.muted_until > now()),
		       count(*), array_agg(DISTINCT rp.reason ORDER BY rp.reason),
		       COALESCE((array_agg(rp.details ORDER BY rp.created_at DESC) FILTER (WHERE rp.details <> ''))[1:5], '{}'),
		       min(rp.created_at), max(rp.created_at),
		       p.deleted_at IS NOT NULL OR c.deleted_at IS NOT NULL
		FROM forum_reports rp
		JOIN forum_posts p ON p.id = rp.post_id
		LEFT JOIN forum_comments c ON c.id = rp.comment_id
		JOIN users a ON a.id = COALESCE(c.user_id, p.user_id)
		WHERE rp.status = 'open'
		GROUP BY rp.post_id, rp.comment_id, p.id, c.id, a.id
		ORDER BY count(*) DESC, max(rp.created_at) DESC
		LIMIT $1 OFFSET $2`, page.Limit+1, page.Offset)
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	defer rows.Close()
	items := make([]reportedItem, 0, page.Limit)
	for rows.Next() {
		var it reportedItem
		if err := rows.Scan(&it.PostID, &it.CommentID, &it.PostTitle, &it.Excerpt, &it.IsAnonymous,
			&it.Author.ID, &it.Author.Handle, &it.Author.DisplayName, &it.Author.MutedUntil,
			&it.Reports, &it.Reasons, &it.Details, &it.FirstAt, &it.LastAt, &it.Removed); err != nil {
			httpx.Internal(w, r, err)
			return
		}
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		httpx.Internal(w, r, err)
		return
	}
	hasMore := len(items) > page.Limit
	if hasMore {
		items = items[:page.Limit]
	}
	var open int
	if err := h.Pool.QueryRow(r.Context(),
		`SELECT count(DISTINCT (post_id, comment_id)) FROM forum_reports WHERE status = 'open'`).Scan(&open); err != nil {
		httpx.Internal(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items, "has_more": hasMore, "open_targets": open})
}

// ResolveForumReports closes every open report on one target:
//
//	action "remove":  delete the post or comment, reports become 'actioned'
//	action "dismiss": leave it, reports become 'dismissed'
//
// mute_days > 0 also mutes the content's author for that many days.
func (h Handler) resolveForumReports(w http.ResponseWriter, r *http.Request) {
	admin, ok := requireAdmin(w, r)
	if !ok {
		return
	}
	var req struct {
		reportTarget
		Action   string `json:"action"`
		MuteDays int    `json:"mute_days"`
		Reason   string `json:"reason"`
	}
	if !httpx.Decode(w, r, &req) {
		return
	}
	if req.Action != "remove" && req.Action != "dismiss" {
		httpx.BadRequest(w, "invalid_action", "Action must be remove or dismiss.")
		return
	}
	if req.MuteDays < 0 || req.MuteDays > muteDaysMax {
		httpx.BadRequest(w, "invalid_days", "Mute for 1-365 days.")
		return
	}
	ctx := r.Context()

	var author uuid.UUID
	err := h.Pool.QueryRow(ctx, `
		SELECT COALESCE(c.user_id, p.user_id) FROM forum_posts p
		LEFT JOIN forum_comments c ON c.id = $2 AND c.post_id = p.id
		WHERE p.id = $1 AND ($2::uuid IS NULL OR c.id IS NOT NULL)`, req.PostID, req.CommentID).Scan(&author)
	if errors.Is(err, pgx.ErrNoRows) {
		forumError(w, http.StatusNotFound, "not_found")
		return
	}
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}

	status := "dismissed"
	if req.Action == "remove" {
		status = "actioned"
		if req.CommentID != nil {
			// Already gone is fine: the reports still need closing.
			if err := softDeleteComment(ctx, h.Pool, *req.CommentID, admin.ID, true); err != nil && !errors.Is(err, pgx.ErrNoRows) {
				httpx.Internal(w, r, err)
				return
			}
		} else if _, err := h.Pool.Exec(ctx,
			`UPDATE forum_posts SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL`, req.PostID); err != nil {
			httpx.Internal(w, r, err)
			return
		}
	}
	tag, err := h.Pool.Exec(ctx, `
		UPDATE forum_reports SET status = $3, resolved_by = $4, resolved_at = now()
		WHERE post_id = $1 AND comment_id IS NOT DISTINCT FROM $2 AND status = 'open'`,
		req.PostID, req.CommentID, status, admin.ID)
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	if req.MuteDays > 0 {
		if err := h.mute(r, author, admin.ID, req.MuteDays, req.Reason); err != nil {
			httpx.Internal(w, r, err)
			return
		}
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"resolved": tag.RowsAffected(), "status": status})
}

// LockForumPost stops (or resumes) new comments on a thread. Admin only.
func (h Handler) lockForumPost(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.PathUUID(w, r, "id")
	if !ok {
		return
	}
	if _, ok := requireAdmin(w, r); !ok {
		return
	}
	var req struct {
		Locked bool `json:"locked"`
	}
	if !httpx.Decode(w, r, &req) {
		return
	}
	tag, err := h.Pool.Exec(r.Context(),
		`UPDATE forum_posts SET is_locked = $2 WHERE id = $1 AND deleted_at IS NULL`, id, req.Locked)
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	if tag.RowsAffected() == 0 {
		forumError(w, http.StatusNotFound, "not_found")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"is_locked": req.Locked})
}

// mute sets (or replaces) a user's mute. Admins can't be muted.
func (h Handler) mute(r *http.Request, userID, by uuid.UUID, days int, reason string) error {
	_, err := h.Pool.Exec(r.Context(), `
		INSERT INTO forum_mutes (user_id, muted_until, reason, muted_by)
		SELECT id, now() + make_interval(days => $2), $3, $4 FROM users WHERE id = $1 AND role <> 'admin'
		ON CONFLICT (user_id) DO UPDATE
		SET muted_until = EXCLUDED.muted_until, reason = EXCLUDED.reason,
		    muted_by = EXCLUDED.muted_by, created_at = now()`,
		userID, days, strings.TrimSpace(reason), by)
	return err
}

type forumMute struct {
	UserID      uuid.UUID `json:"user_id"`
	Handle      string    `json:"handle"`
	DisplayName string    `json:"display_name"`
	MutedUntil  time.Time `json:"muted_until"`
	Reason      string    `json:"reason"`
	MutedBy     string    `json:"muted_by"`
	CreatedAt   time.Time `json:"created_at"`
}

// ListForumMutes returns mutes still in effect, ending soonest first.
func (h Handler) listForumMutes(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdmin(w, r); !ok {
		return
	}
	rows, err := h.Pool.Query(r.Context(), `
		SELECT m.user_id, u.handle, u.display_name, m.muted_until, m.reason,
		       COALESCE(b.display_name, b.handle, ''), m.created_at
		FROM forum_mutes m
		JOIN users u ON u.id = m.user_id
		LEFT JOIN users b ON b.id = m.muted_by
		WHERE m.muted_until > now()
		ORDER BY m.muted_until
		LIMIT 200`)
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	mutes, err := pgx.CollectRows(rows, pgx.RowToStructByPos[forumMute])
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"mutes": mutes})
}

// MuteForumUser mutes a user by id or handle for 1-365 days.
func (h Handler) muteForumUser(w http.ResponseWriter, r *http.Request) {
	admin, ok := requireAdmin(w, r)
	if !ok {
		return
	}
	var req struct {
		UserID *uuid.UUID `json:"user_id"`
		Handle string     `json:"handle"`
		Days   int        `json:"days"`
		Reason string     `json:"reason"`
	}
	if !httpx.Decode(w, r, &req) {
		return
	}
	if req.Days < 1 || req.Days > muteDaysMax {
		httpx.BadRequest(w, "invalid_days", "Mute for 1-365 days.")
		return
	}
	var userID uuid.UUID
	var role string
	err := h.Pool.QueryRow(r.Context(), `
		SELECT id, role FROM users
		WHERE ($1::uuid IS NOT NULL AND id = $1) OR ($1::uuid IS NULL AND lower(handle) = lower($2))`,
		req.UserID, strings.TrimPrefix(strings.TrimSpace(req.Handle), "@")).Scan(&userID, &role)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.NotFound(w, "No user with that handle.")
		return
	}
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	if role == "admin" {
		httpx.BadRequest(w, "cannot_mute_admin", "Moderators can't be muted.")
		return
	}
	if err := h.mute(r, userID, admin.ID, req.Days, req.Reason); err != nil {
		httpx.Internal(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h Handler) unmuteForumUser(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.PathUUID(w, r, "id")
	if !ok {
		return
	}
	if _, ok := requireAdmin(w, r); !ok {
		return
	}
	if _, err := h.Pool.Exec(r.Context(), `DELETE FROM forum_mutes WHERE user_id = $1`, id); err != nil {
		httpx.Internal(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
