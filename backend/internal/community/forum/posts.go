package forum

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"codritium/backend/internal/community/votes"
	"codritium/backend/internal/platform/httpx"

	"codritium/backend/internal/problems"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"codritium/backend/internal/auth"
)

// ListForumPosts returns one page of the feed.
//
//	section: interview|career|compensation|feedback|problems (empty = all, "For You")
//	sort:    hot (default) | votes | newest
//	q:       full-text search over title and body, plus a title substring match
//	tag:     exact tag
func (h Handler) listForumPosts(w http.ResponseWriter, r *http.Request) {
	viewer := auth.FromContext(r.Context())
	qs := r.URL.Query()
	page := httpx.ParsePage(r, forumPageDefault, forumPageMax, forumOffsetMax)
	limit, offset := page.Limit, page.Offset

	where := []string{"p.deleted_at IS NULL"}
	args := []any{viewerID(viewer)}
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
		         / power(extract(epoch FROM now() - p.created_at) / 3600 + 2, 1.5) DESC,
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
	httpx.JSON(w, http.StatusOK, map[string]any{"posts": posts, "has_more": hasMore})
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
	if !ok || !h.allow(w, r, u, postLimit) {
		return
	}
	var id uuid.UUID
	if err := h.Pool.QueryRow(r.Context(), `
		INSERT INTO forum_posts (user_id, section, problem_slug, title, body_md, tags, is_anonymous)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id`,
		u.ID, req.Section, req.ProblemSlug, req.Title, req.BodyMD, req.Tags, req.IsAnonymous,
	).Scan(&id); err != nil {
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
	if !ok {
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
	if !ok || !h.allow(w, r, u, voteLimit) {
		return
	}
	votes.Handle(w, r, h.Pool, votes.ForumPost, id, u.ID)
}
