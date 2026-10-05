package forum

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"codritium/backend/internal/community/votes"
	"codritium/backend/internal/notifications"
	"codritium/backend/internal/platform/httpx"

	"codritium/backend/internal/problems"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"codritium/backend/internal/auth"
)

// ListForumPosts returns one page of the feed.
//
//	section: one of forumSections (empty = all, "For You")
//	sort:    hot (default) | votes | newest
//	q:       full-text search over title and body, plus a title substring match
//	tag:     exact tag
//	problem: posts linking that problem slug
//	as_of:   the snapshot time from the first page's response
//
// Paging is by offset over a snapshot: every page of one scroll uses the
// first page's as_of, so posts written since don't push others down a page
// and "hot" scores don't decay between pages (both used to skip posts).
func (h Handler) listForumPosts(w http.ResponseWriter, r *http.Request) {
	viewer := auth.FromContext(r.Context())
	qs := r.URL.Query()
	page := httpx.ParsePage(r, forumPageDefault, forumPageMax, forumOffsetMax)
	limit, offset := page.Limit, page.Offset

	// The snapshot comes from the database clock, which stamps created_at,
	// so a post written a moment ago is never just outside the first page.
	var asOf time.Time
	if s := qs.Get("as_of"); s != "" {
		t, err := time.Parse(time.RFC3339Nano, s)
		if err != nil {
			httpx.BadRequest(w, "invalid_as_of", "as_of must be an RFC 3339 time from a previous page.")
			return
		}
		asOf = t
	} else if err := h.Pool.QueryRow(r.Context(), `SELECT now()`).Scan(&asOf); err != nil {
		httpx.Internal(w, r, err)
		return
	}

	args := []any{viewerID(viewer), asOf}
	where := []string{"p.deleted_at IS NULL", "p.created_at <= $2"}
	arg := func(v any) string {
		args = append(args, v)
		return "$" + strconv.Itoa(len(args))
	}
	if s := qs.Get("section"); s != "" {
		if !forumSections[s] {
			forumError(w, http.StatusBadRequest, "invalid_section")
			return
		}
		where = append(where, "p.section = "+arg(s))
	}
	if tag := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(qs.Get("tag")), "#")); tag != "" {
		where = append(where, arg(tag)+" = ANY(p.tags)")
	}
	if slug := strings.TrimSpace(qs.Get("problem")); slug != "" {
		where = append(where, "p.problem_slug = "+arg(slug))
	}
	if q := strings.TrimSpace(qs.Get("q")); q != "" {
		if utf8.RuneCountInString(q) > 100 {
			q = string([]rune(q)[:100])
		}
		where = append(where, fmt.Sprintf(
			"(p.search_vector @@ websearch_to_tsquery('english', %s) OR p.title ILIKE %s)",
			arg(q), arg("%"+escapeLike(q)+"%")))
	}

	var order string
	switch qs.Get("sort") {
	case "", "hot":
		// Hacker-News-style gravity: engagement decays with age so the
		// "For You" feed keeps moving.
		order = `(p.upvotes - p.downvotes + p.comment_count * 0.5 + ln(1 + p.view_count) * 0.25 + 1)
		         / power(extract(epoch FROM $2::timestamptz - p.created_at) / 3600 + 2, 1.5) DESC,
		         p.created_at DESC, p.id DESC`
	case "votes":
		order = "(p.upvotes - p.downvotes) DESC, p.created_at DESC, p.id DESC"
	case "newest":
		order = "p.created_at DESC, p.id DESC"
	default:
		forumError(w, http.StatusBadRequest, "invalid_sort")
		return
	}

	query := `SELECT ` + forumPostColumns("left(p.body_md, 800)") + forumPostFrom +
		` WHERE ` + strings.Join(where, " AND ") +
		` ORDER BY ` + order +
		` LIMIT ` + arg(limit+1) + ` OFFSET ` + arg(offset)
	rows, err := h.Pool.Query(r.Context(), query, args...)
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	defer rows.Close()

	posts := make([]forumPost, 0, limit)
	for rows.Next() {
		p, err := scanForumPost(rows, viewer, false)
		if err != nil {
			httpx.Internal(w, r, err)
			return
		}
		posts = append(posts, p)
	}
	if err := rows.Err(); err != nil {
		httpx.Internal(w, r, err)
		return
	}
	hasMore := len(posts) > limit
	if hasMore {
		posts = posts[:limit]
	}
	httpx.JSON(w, http.StatusOK, map[string]any{
		"posts": posts, "has_more": hasMore, "as_of": asOf.UTC().Format(time.RFC3339Nano),
	})
}

// ListPinnedForumPosts returns admin-pinned posts for the top of the feed.
func (h Handler) listPinnedForumPosts(w http.ResponseWriter, r *http.Request) {
	viewer := auth.FromContext(r.Context())
	rows, err := h.Pool.Query(r.Context(), `SELECT `+forumPostColumns("left(p.body_md, 800)")+forumPostFrom+`
		WHERE p.is_pinned AND p.deleted_at IS NULL
		ORDER BY p.created_at DESC
		LIMIT $2`, viewerID(viewer), forumPinnedMax)
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	defer rows.Close()
	posts := []forumPost{}
	for rows.Next() {
		p, err := scanForumPost(rows, viewer, false)
		if err != nil {
			httpx.Internal(w, r, err)
			return
		}
		posts = append(posts, p)
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"posts": posts})
}

// ListTrendingForumPosts returns the sidebar's top posts: posts people have
// actually read, most viewed in the last 7 days first, then older ones.
func (h Handler) listTrendingForumPosts(w http.ResponseWriter, r *http.Request) {
	rows, err := h.Pool.Query(r.Context(), `
		SELECT id, title, section, COALESCE(tags[1], ''), view_count
		FROM forum_posts
		WHERE deleted_at IS NULL AND view_count > 0
		ORDER BY (created_at > now() - interval '7 days') DESC,
		         view_count DESC, (upvotes - downvotes) DESC, created_at DESC
		LIMIT $1`, forumTrendingMax)
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	defer rows.Close()
	items := []forumTrendingItem{}
	for rows.Next() {
		var it forumTrendingItem
		if err := rows.Scan(&it.ID, &it.Title, &it.Section, &it.Tag, &it.ViewCount); err != nil {
			httpx.Internal(w, r, err)
			return
		}
		items = append(items, it)
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"posts": items})
}

// GetForumPost returns one post with its full body. Reading doesn't count a
// view: the page reports one separately (recordForumView), so server-side
// renders and link prefetches don't inflate the count.
func (h Handler) getForumPost(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.PathUUID(w, r, "id")
	if !ok {
		return
	}
	viewer := auth.FromContext(r.Context())
	p, err := loadForumPost(r.Context(), h.Pool, id, viewer)
	if errors.Is(err, pgx.ErrNoRows) {
		forumError(w, http.StatusNotFound, "not_found")
		return
	}
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, p)
}

type forumPostRequest struct {
	Section     string   `json:"section"`
	Title       string   `json:"title"`
	BodyMD      string   `json:"body_md"`
	Tags        []string `json:"tags"`
	IsAnonymous bool     `json:"is_anonymous"`
	ProblemSlug *string  `json:"problem_slug"`
}

// validate normalizes the request in place and returns an error code for the
// client, or "" when the post is acceptable.
func (req *forumPostRequest) validate(ctx context.Context, pool *pgxpool.Pool) string {
	req.Title = strings.TrimSpace(req.Title)
	req.BodyMD = strings.TrimSpace(req.BodyMD)
	if !forumSections[req.Section] {
		return "invalid_section"
	}
	if n := utf8.RuneCountInString(req.Title); n < forumTitleMin || n > forumTitleMax {
		return "invalid_title"
	}
	if req.BodyMD == "" || utf8.RuneCountInString(req.BodyMD) > forumBodyMax {
		return "invalid_body"
	}
	tags, err := normalizeForumTags(req.Tags)
	if err != nil {
		return err.Error()
	}
	req.Tags = tags
	if req.IsAnonymous && !forumAnonSections[req.Section] {
		return "anonymous_not_allowed"
	}
	if req.ProblemSlug != nil {
		slug := strings.TrimSpace(*req.ProblemSlug)
		if slug == "" {
			req.ProblemSlug = nil
		} else {
			if exists, err := (problems.Store{Pool: pool}).Exists(ctx, slug); err != nil || !exists {
				return "unknown_problem"
			}
			req.ProblemSlug = &slug
		}
	}
	return ""
}

func decodePostRequest(w http.ResponseWriter, r *http.Request, pool *pgxpool.Pool) (forumPostRequest, bool) {
	var req forumPostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		forumError(w, http.StatusBadRequest, "bad_json")
		return req, false
	}
	if code := req.validate(r.Context(), pool); code != "" {
		forumError(w, http.StatusBadRequest, code)
		return req, false
	}
	return req, true
}

func (h Handler) createForumPost(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.Require(w, r)
	if !ok {
		return
	}
	req, ok := decodePostRequest(w, r, h.Pool)
	if !ok || !h.allow(w, r, u, postLimit, req.Title+"\n"+req.BodyMD) {
		return
	}
	// The post, its author's follow, and mention notifications land together.
	tx, err := h.Pool.BeginTx(r.Context(), pgx.TxOptions{})
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	defer tx.Rollback(r.Context())
	var id uuid.UUID
	if err := tx.QueryRow(r.Context(), `
		INSERT INTO forum_posts (user_id, section, problem_slug, title, body_md, tags, is_anonymous)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id`,
		u.ID, req.Section, req.ProblemSlug, req.Title, req.BodyMD, req.Tags, req.IsAnonymous,
	).Scan(&id); err != nil {
		httpx.Internal(w, r, err)
		return
	}
	if _, err := tx.Exec(r.Context(),
		`INSERT INTO forum_post_follows (post_id, user_id) VALUES ($1, $2)`, id, u.ID); err != nil {
		httpx.Internal(w, r, err)
		return
	}
	if err := notifyNewPost(r.Context(), tx, id, u.ID, req.BodyMD, req.IsAnonymous); err != nil {
		httpx.Internal(w, r, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		httpx.Internal(w, r, err)
		return
	}
	p, err := loadForumPost(r.Context(), h.Pool, id, u)
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, p)
}

// UpdateForumPost replaces an existing post's content. Only the author may
// edit; anyone else gets the same 404 as a missing post.
func (h Handler) updateForumPost(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.PathUUID(w, r, "id")
	if !ok {
		return
	}
	u, ok := auth.Require(w, r)
	if !ok {
		return
	}
	req, ok := decodePostRequest(w, r, h.Pool)
	if !ok || !h.allow(w, r, u, editLimit, "") {
		return
	}
	tag, err := h.Pool.Exec(r.Context(), `
		UPDATE forum_posts
		SET section = $3, problem_slug = $4, title = $5, body_md = $6, tags = $7,
		    is_anonymous = $8, updated_at = now()
		WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL`,
		id, u.ID, req.Section, req.ProblemSlug, req.Title, req.BodyMD, req.Tags, req.IsAnonymous)
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	if tag.RowsAffected() == 0 {
		forumError(w, http.StatusNotFound, "not_found")
		return
	}
	p, err := loadForumPost(r.Context(), h.Pool, id, u)
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, p)
}

// DeleteForumPost soft-deletes a post. Authors can delete their own posts;
// admins can delete any.
func (h Handler) deleteForumPost(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.PathUUID(w, r, "id")
	if !ok {
		return
	}
	u, ok := auth.Require(w, r)
	if !ok {
		return
	}
	tag, err := h.Pool.Exec(r.Context(), `
		UPDATE forum_posts SET deleted_at = now()
		WHERE id = $1 AND deleted_at IS NULL AND (user_id = $2 OR $3)`,
		id, u.ID, u.Role == "admin")
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	if tag.RowsAffected() == 0 {
		forumError(w, http.StatusNotFound, "not_found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// PinForumPost toggles whether a post is featured at the top of the feed.
// Admin only.
func (h Handler) pinForumPost(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.PathUUID(w, r, "id")
	if !ok {
		return
	}
	u, ok := auth.Require(w, r)
	if !ok {
		return
	}
	if u.Role != "admin" {
		forumError(w, http.StatusForbidden, "admin_only")
		return
	}
	var req struct {
		Pinned bool `json:"pinned"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		forumError(w, http.StatusBadRequest, "bad_json")
		return
	}
	tag, err := h.Pool.Exec(r.Context(),
		`UPDATE forum_posts SET is_pinned = $2 WHERE id = $1 AND deleted_at IS NULL`, id, req.Pinned)
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	if tag.RowsAffected() == 0 {
		forumError(w, http.StatusNotFound, "not_found")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"is_pinned": req.Pinned})
}

func (h Handler) voteForumPost(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.PathUUID(w, r, "id")
	if !ok {
		return
	}
	u, ok := auth.Require(w, r)
	if !ok || !h.allow(w, r, u, voteLimit, "") {
		return
	}
	value, ok := votes.DecodeValue(w, r)
	if !ok {
		return
	}
	res, err := votes.Apply(r.Context(), h.Pool, votes.ForumPost, id, u.ID, value)
	if errors.Is(err, votes.ErrNotFound) {
		forumError(w, http.StatusNotFound, "not_found")
		return
	}
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	// An upvote that lifts the score to a milestone tells the author, once
	// per milestone. A failure here shouldn't fail the vote itself.
	if m := reachedMilestone(res.Score); value == 1 && m > 0 {
		if err := h.notifyMilestone(r.Context(), id, m); err != nil {
			log.Printf("forum: milestone notification for post %s: %v", id, err)
		}
	}
	httpx.JSON(w, http.StatusOK, res)
}

func (h Handler) notifyMilestone(ctx context.Context, postID uuid.UUID, milestone int) error {
	var author uuid.UUID
	if err := h.Pool.QueryRow(ctx, `SELECT user_id FROM forum_posts WHERE id = $1`, postID).Scan(&author); err != nil {
		return err
	}
	return notifications.InsertMilestone(ctx, h.Pool, author, postID, milestone)
}
