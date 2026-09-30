package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"codritium/backend/internal/problems"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"codritium/backend/internal/auth"
)

// Community forum (/forums). Reading is public — the auth middleware leaves
// the user nil for signed-out visitors — while posting, voting and
// commenting require a session. Anonymous posts hide their author from
// everyone except the author themselves and admins.

type ForumDeps struct {
	Pool *pgxpool.Pool
}

var forumSections = map[string]bool{
	"interview":    true,
	"career":       true,
	"compensation": true,
	"feedback":     true,
	"problems":     true,
}

// Sections where posts (and comments on them) may be anonymous.
var forumAnonSections = map[string]bool{
	"interview":    true,
	"compensation": true,
}

const (
	forumTitleMin    = 5
	forumTitleMax    = 150
	forumBodyMax     = 20000
	forumCommentMax  = 5000
	forumMaxTags     = 5
	forumExcerptLen  = 200
	forumPageDefault = 20
	forumPageMax     = 50
	forumOffsetMax   = 2000
	forumPinnedMax   = 5
	forumTrendingMax = 10
)

var forumTagPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9+#.-]{0,23}$`)

type forumAuthor struct {
	ID          uuid.UUID `json:"id"`
	Handle      string    `json:"handle"`
	DisplayName string    `json:"display_name"`
	AvatarURL   string    `json:"avatar_url"`
	AvatarColor string    `json:"avatar_color"`
	// Verified marks official (admin) accounts, like LeetCode's check badge.
	Verified bool `json:"verified"`
}

type forumPost struct {
	ID           uuid.UUID    `json:"id"`
	Section      string       `json:"section"`
	ProblemSlug  *string      `json:"problem_slug"`
	Title        string       `json:"title"`
	Excerpt      string       `json:"excerpt"`
	BodyMD       string       `json:"body_md,omitempty"`
	Tags         []string     `json:"tags"`
	IsAnonymous  bool         `json:"is_anonymous"`
	IsPinned     bool         `json:"is_pinned"`
	Author       *forumAuthor `json:"author"`
	IsMine       bool         `json:"is_mine"`
	Upvotes      int          `json:"upvotes"`
	Downvotes    int          `json:"downvotes"`
	Score        int          `json:"score"`
	ViewCount    int          `json:"view_count"`
	CommentCount int          `json:"comment_count"`
	MyVote       int          `json:"my_vote"`
	CreatedAt    time.Time    `json:"created_at"`
	UpdatedAt    time.Time    `json:"updated_at"`
}

type forumComment struct {
	ID          uuid.UUID      `json:"id"`
	PostID      uuid.UUID      `json:"post_id"`
	ParentID    *uuid.UUID     `json:"parent_id"`
	Body        string         `json:"body"`
	IsAnonymous bool           `json:"is_anonymous"`
	Author      *forumAuthor   `json:"author"`
	IsMine      bool           `json:"is_mine"`
	IsOP        bool           `json:"is_op"`
	Upvotes     int            `json:"upvotes"`
	Downvotes   int            `json:"downvotes"`
	Score       int            `json:"score"`
	MyVote      int            `json:"my_vote"`
	CreatedAt   time.Time      `json:"created_at"`
	Replies     []forumComment `json:"replies,omitempty"`
}

type forumTrendingItem struct {
	ID        uuid.UUID `json:"id"`
	Title     string    `json:"title"`
	Section   string    `json:"section"`
	Tag       string    `json:"tag"`
	ViewCount int       `json:"view_count"`
}

// --- shared helpers ---------------------------------------------------

func forumError(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, map[string]any{"error": code})
}

func viewerID(u *auth.User) uuid.UUID {
	if u == nil {
		return uuid.Nil
	}
	return u.ID
}

// canSeeAuthor reports whether the viewer may see who wrote an item.
func canSeeAuthor(viewer *auth.User, authorID uuid.UUID, anonymous bool) bool {
	if !anonymous {
		return true
	}
	return viewer != nil && (viewer.ID == authorID || viewer.Role == "admin")
}

// Column list shared by every post query; bodyExpr picks the full body or a
// prefix long enough to build an excerpt from.
func forumPostColumns(bodyExpr string) string {
	return `p.id, p.user_id, p.section, p.problem_slug, p.title, ` + bodyExpr + `,
	        p.tags, p.is_anonymous, p.is_pinned, p.upvotes, p.downvotes,
	        p.view_count, p.comment_count, p.created_at, p.updated_at,
	        u.handle, u.display_name, COALESCE(u.avatar_url,''), COALESCE(u.avatar_color,''),
	        COALESCE(u.role,''), COALESCE(v.value, 0)`
}

// forumPostFrom joins author and the viewer's vote ($1 is the viewer id).
const forumPostFrom = `
	FROM forum_posts p
	JOIN users u ON u.id = p.user_id
	LEFT JOIN forum_post_votes v ON v.post_id = p.id AND v.user_id = $1`

func scanForumPost(row pgx.Row, viewer *auth.User, full bool) (forumPost, error) {
	var (
		p        forumPost
		authorID uuid.UUID
		body     string
		a        forumAuthor
		role     string
	)
	if err := row.Scan(&p.ID, &authorID, &p.Section, &p.ProblemSlug, &p.Title, &body,
		&p.Tags, &p.IsAnonymous, &p.IsPinned, &p.Upvotes, &p.Downvotes,
		&p.ViewCount, &p.CommentCount, &p.CreatedAt, &p.UpdatedAt,
		&a.Handle, &a.DisplayName, &a.AvatarURL, &a.AvatarColor, &role, &p.MyVote); err != nil {
		return forumPost{}, err
	}
	p.Score = p.Upvotes - p.Downvotes
	p.Excerpt = forumExcerpt(body)
	if full {
		p.BodyMD = body
	}
	if p.Tags == nil {
		p.Tags = []string{}
	}
	p.IsMine = viewer != nil && viewer.ID == authorID
	if canSeeAuthor(viewer, authorID, p.IsAnonymous) {
		a.ID = authorID
		a.Verified = role == "admin"
		p.Author = &a
	}
	return p, nil
}

func loadForumPost(ctx context.Context, pool *pgxpool.Pool, id uuid.UUID, viewer *auth.User) (forumPost, error) {
	row := pool.QueryRow(ctx, `SELECT `+forumPostColumns("p.body_md")+forumPostFrom+`
		WHERE p.id = $2 AND p.deleted_at IS NULL`, viewerID(viewer), id)
	return scanForumPost(row, viewer, true)
}

var (
	mdFence     = regexp.MustCompile("(?s)```.*?(```|$)")
	mdLink      = regexp.MustCompile(`!?\[([^\]]*)\]\([^)]*\)`)
	mdLineStart = regexp.MustCompile(`(?m)^\s{0,3}(#{1,6}\s+|>\s?|[-*+]\s+|\d+\.\s+)`)
	mdEmphasis  = regexp.MustCompile("[*_`~]+")
	mdSpace     = regexp.MustCompile(`\s+`)
)

// forumExcerpt turns a markdown body into a short plain-text preview for
// feed cards.
func forumExcerpt(md string) string {
	s := mdFence.ReplaceAllString(md, " ")
	s = mdLink.ReplaceAllString(s, "$1")
	s = mdLineStart.ReplaceAllString(s, "")
	s = mdEmphasis.ReplaceAllString(s, "")
	s = strings.TrimSpace(mdSpace.ReplaceAllString(s, " "))
	if utf8.RuneCountInString(s) <= forumExcerptLen {
		return s
	}
	r := []rune(s)
	return strings.TrimSpace(string(r[:forumExcerptLen])) + "…"
}

func normalizeForumTags(in []string) ([]string, error) {
	out := make([]string, 0, len(in))
	seen := map[string]bool{}
	for _, raw := range in {
		t := strings.ToLower(strings.Join(strings.Fields(raw), "-"))
		t = strings.TrimPrefix(t, "#")
		if t == "" || seen[t] {
			continue
		}
		if !forumTagPattern.MatchString(t) {
			return nil, fmt.Errorf("invalid_tag")
		}
		seen[t] = true
		out = append(out, t)
	}
	if len(out) > forumMaxTags {
		return nil, fmt.Errorf("too_many_tags")
	}
	return out, nil
}

func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

func pageParams(r *http.Request) (limit, offset int) {
	limit = forumPageDefault
	if n, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && n > 0 && n <= forumPageMax {
		limit = n
	}
	if n, err := strconv.Atoi(r.URL.Query().Get("offset")); err == nil && n > 0 {
		offset = min(n, forumOffsetMax)
	}
	return limit, offset
}

// applyForumVote upserts the caller's vote on a post or comment and updates
// the cached counters in one transaction. Table and column names are fixed
// by the two callers below, never taken from the request.
func applyForumVote(ctx context.Context, pool *pgxpool.Pool, voteTable, targetCol, targetTable string, targetID, userID uuid.UUID, value int) (up, down int, err error) {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback(ctx)

	var exists bool
	if err := tx.QueryRow(ctx, fmt.Sprintf(
		`SELECT EXISTS(SELECT 1 FROM %s WHERE id = $1 AND deleted_at IS NULL)`, targetTable),
		targetID).Scan(&exists); err != nil {
		return 0, 0, err
	}
	if !exists {
		return 0, 0, pgx.ErrNoRows
	}

	var prev int
	err = tx.QueryRow(ctx, fmt.Sprintf(
		`SELECT value FROM %s WHERE %s = $1 AND user_id = $2 FOR UPDATE`, voteTable, targetCol),
		targetID, userID).Scan(&prev)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return 0, 0, err
	}
	switch {
	case value == 0 && prev != 0:
		_, err = tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s WHERE %s = $1 AND user_id = $2`, voteTable, targetCol),
			targetID, userID)
	case value != 0 && prev == 0:
		_, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s (%s, user_id, value) VALUES ($1, $2, $3)`, voteTable, targetCol),
			targetID, userID, value)
	case value != 0 && prev != value:
		_, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s SET value = $3, voted_at = now() WHERE %s = $1 AND user_id = $2`, voteTable, targetCol),
			targetID, userID, value)
	}
	if err != nil {
		return 0, 0, err
	}
	upDelta, downDelta := voteDeltas(prev, value)
	if err := tx.QueryRow(ctx, fmt.Sprintf(
		`UPDATE %s SET upvotes = upvotes + $2, downvotes = downvotes + $3 WHERE id = $1 RETURNING upvotes, downvotes`, targetTable),
		targetID, upDelta, downDelta).Scan(&up, &down); err != nil {
		return 0, 0, err
	}
	return up, down, tx.Commit(ctx)
}

func decodeVote(w http.ResponseWriter, r *http.Request) (int, bool) {
	var req voteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		forumError(w, http.StatusBadRequest, "bad_json")
		return 0, false
	}
	if req.Value != -1 && req.Value != 0 && req.Value != 1 {
		forumError(w, http.StatusBadRequest, "invalid_vote")
		return 0, false
	}
	return req.Value, true
}

func pathUUID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		forumError(w, http.StatusNotFound, "not_found")
		return uuid.Nil, false
	}
	return id, true
}

func requireUser(w http.ResponseWriter, r *http.Request) (*auth.User, bool) {
	u := auth.FromContext(r.Context())
	if u == nil {
		forumError(w, http.StatusUnauthorized, "login_required")
		return nil, false
	}
	return u, true
}

// --- posts ------------------------------------------------------------

// ListForumPosts returns one page of the feed.
//
//	section: interview|career|compensation|feedback|problems (empty = all, "For You")
//	sort:    hot (default) | votes | newest
//	q:       full-text search over title and body, plus a title substring match
//	tag:     exact tag
func ListForumPosts(deps ForumDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		viewer := auth.FromContext(r.Context())
		qs := r.URL.Query()
		limit, offset := pageParams(r)

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
		rows, err := deps.Pool.Query(r.Context(), query, args...)
		if err != nil {
			http.Error(w, "list posts: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		posts := make([]forumPost, 0, limit)
		for rows.Next() {
			p, err := scanForumPost(rows, viewer, false)
			if err != nil {
				http.Error(w, "scan post: "+err.Error(), http.StatusInternalServerError)
				return
			}
			posts = append(posts, p)
		}
		if err := rows.Err(); err != nil {
			http.Error(w, "list posts: "+err.Error(), http.StatusInternalServerError)
			return
		}
		hasMore := len(posts) > limit
		if hasMore {
			posts = posts[:limit]
		}
		writeJSON(w, http.StatusOK, map[string]any{"posts": posts, "has_more": hasMore})
	}
}

// ListPinnedForumPosts returns admin-pinned posts for the top of the feed.
func ListPinnedForumPosts(deps ForumDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		viewer := auth.FromContext(r.Context())
		rows, err := deps.Pool.Query(r.Context(), `SELECT `+forumPostColumns("left(p.body_md, 800)")+forumPostFrom+`
			WHERE p.is_pinned AND p.deleted_at IS NULL
			ORDER BY p.created_at DESC
			LIMIT $2`, viewerID(viewer), forumPinnedMax)
		if err != nil {
			http.Error(w, "list pinned: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer rows.Close()
		posts := []forumPost{}
		for rows.Next() {
			p, err := scanForumPost(rows, viewer, false)
			if err != nil {
				http.Error(w, "scan post: "+err.Error(), http.StatusInternalServerError)
				return
			}
			posts = append(posts, p)
		}
		writeJSON(w, http.StatusOK, map[string]any{"posts": posts})
	}
}

// ListTrendingForumPosts returns the sidebar's top posts: posts people have
// actually read, most viewed in the last 7 days first, then older ones.
func ListTrendingForumPosts(deps ForumDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := deps.Pool.Query(r.Context(), `
			SELECT id, title, section, COALESCE(tags[1], ''), view_count
			FROM forum_posts
			WHERE deleted_at IS NULL AND view_count > 0
			ORDER BY (created_at > now() - interval '7 days') DESC,
			         view_count DESC, (upvotes - downvotes) DESC, created_at DESC
			LIMIT $1`, forumTrendingMax)
		if err != nil {
			http.Error(w, "trending: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer rows.Close()
		items := []forumTrendingItem{}
		for rows.Next() {
			var it forumTrendingItem
			if err := rows.Scan(&it.ID, &it.Title, &it.Section, &it.Tag, &it.ViewCount); err != nil {
				http.Error(w, "scan trending: "+err.Error(), http.StatusInternalServerError)
				return
			}
			items = append(items, it)
		}
		writeJSON(w, http.StatusOK, map[string]any{"posts": items})
	}
}

// GetForumPost returns one post with its full body and records a view for
// signed-in viewers (once per user).
func GetForumPost(deps ForumDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathUUID(w, r)
		if !ok {
			return
		}
		viewer := auth.FromContext(r.Context())
		if viewer != nil {
			if _, err := deps.Pool.Exec(r.Context(), `
				WITH ins AS (
				  INSERT INTO forum_post_views (post_id, user_id)
				  SELECT $1, $2 WHERE EXISTS (SELECT 1 FROM forum_posts WHERE id = $1 AND deleted_at IS NULL)
				  ON CONFLICT DO NOTHING
				  RETURNING post_id
				)
				UPDATE forum_posts SET view_count = view_count + 1
				WHERE id IN (SELECT post_id FROM ins)`, id, viewer.ID); err != nil {
				http.Error(w, "record view: "+err.Error(), http.StatusInternalServerError)
				return
			}
		}
		p, err := loadForumPost(r.Context(), deps.Pool, id, viewer)
		if errors.Is(err, pgx.ErrNoRows) {
			forumError(w, http.StatusNotFound, "not_found")
			return
		}
		if err != nil {
			http.Error(w, "get post: "+err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, p)
	}
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
		if slug == "" || req.Section != "problems" {
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

func CreateForumPost(deps ForumDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, ok := requireUser(w, r)
		if !ok {
			return
		}
		req, ok := decodePostRequest(w, r, deps.Pool)
		if !ok {
			return
		}
		var id uuid.UUID
		if err := deps.Pool.QueryRow(r.Context(), `
			INSERT INTO forum_posts (user_id, section, problem_slug, title, body_md, tags, is_anonymous)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			RETURNING id`,
			u.ID, req.Section, req.ProblemSlug, req.Title, req.BodyMD, req.Tags, req.IsAnonymous,
		).Scan(&id); err != nil {
			http.Error(w, "create post: "+err.Error(), http.StatusInternalServerError)
			return
		}
		p, err := loadForumPost(r.Context(), deps.Pool, id, u)
		if err != nil {
			http.Error(w, "load post: "+err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusCreated, p)
	}
}

// UpdateForumPost replaces an existing post's content. Only the author may
// edit; anyone else gets the same 404 as a missing post.
func UpdateForumPost(deps ForumDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathUUID(w, r)
		if !ok {
			return
		}
		u, ok := requireUser(w, r)
		if !ok {
			return
		}
		req, ok := decodePostRequest(w, r, deps.Pool)
		if !ok {
			return
		}
		tag, err := deps.Pool.Exec(r.Context(), `
			UPDATE forum_posts
			SET section = $3, problem_slug = $4, title = $5, body_md = $6, tags = $7,
			    is_anonymous = $8, updated_at = now()
			WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL`,
			id, u.ID, req.Section, req.ProblemSlug, req.Title, req.BodyMD, req.Tags, req.IsAnonymous)
		if err != nil {
			http.Error(w, "update post: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if tag.RowsAffected() == 0 {
			forumError(w, http.StatusNotFound, "not_found")
			return
		}
		p, err := loadForumPost(r.Context(), deps.Pool, id, u)
		if err != nil {
			http.Error(w, "load post: "+err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, p)
	}
}

// DeleteForumPost soft-deletes a post. Authors can delete their own posts;
// admins can delete any.
func DeleteForumPost(deps ForumDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathUUID(w, r)
		if !ok {
			return
		}
		u, ok := requireUser(w, r)
		if !ok {
			return
		}
		tag, err := deps.Pool.Exec(r.Context(), `
			UPDATE forum_posts SET deleted_at = now()
			WHERE id = $1 AND deleted_at IS NULL AND (user_id = $2 OR $3)`,
			id, u.ID, u.Role == "admin")
		if err != nil {
			http.Error(w, "delete post: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if tag.RowsAffected() == 0 {
			forumError(w, http.StatusNotFound, "not_found")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// PinForumPost toggles whether a post is featured at the top of the feed.
// Admin only.
func PinForumPost(deps ForumDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathUUID(w, r)
		if !ok {
			return
		}
		u, ok := requireUser(w, r)
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
		tag, err := deps.Pool.Exec(r.Context(),
			`UPDATE forum_posts SET is_pinned = $2 WHERE id = $1 AND deleted_at IS NULL`, id, req.Pinned)
		if err != nil {
			http.Error(w, "pin post: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if tag.RowsAffected() == 0 {
			forumError(w, http.StatusNotFound, "not_found")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"is_pinned": req.Pinned})
	}
}

func VoteForumPost(deps ForumDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathUUID(w, r)
		if !ok {
			return
		}
		u, ok := requireUser(w, r)
		if !ok {
			return
		}
		value, ok := decodeVote(w, r)
		if !ok {
			return
		}
		up, down, err := applyForumVote(r.Context(), deps.Pool, "forum_post_votes", "post_id", "forum_posts", id, u.ID, value)
		if errors.Is(err, pgx.ErrNoRows) {
			forumError(w, http.StatusNotFound, "not_found")
			return
		}
		if err != nil {
			http.Error(w, "vote: "+err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"upvotes": up, "downvotes": down, "score": up - down, "my_vote": value})
	}
}

// --- comments ---------------------------------------------------------

const forumCommentColumns = `
	c.id, c.post_id, c.parent_id, c.user_id, c.body, c.is_anonymous,
	c.upvotes, c.downvotes, c.created_at,
	u.handle, u.display_name, COALESCE(u.avatar_url,''), COALESCE(u.avatar_color,''),
	COALESCE(u.role,''), COALESCE(v.value, 0)`

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
		&c.Upvotes, &c.Downvotes, &c.CreatedAt,
		&a.Handle, &a.DisplayName, &a.AvatarURL, &a.AvatarColor, &role, &c.MyVote); err != nil {
		return forumComment{}, err
	}
	c.Score = c.Upvotes - c.Downvotes
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
}

func loadForumPostMeta(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, id uuid.UUID) (forumPostMeta, error) {
	var m forumPostMeta
	err := q.QueryRow(ctx,
		`SELECT user_id, section FROM forum_posts WHERE id = $1 AND deleted_at IS NULL`, id,
	).Scan(&m.authorID, &m.section)
	return m, err
}

// ListForumComments returns a page of top-level comments (sort=best|newest)
// with all of their replies, oldest reply first.
func ListForumComments(deps ForumDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		postID, ok := pathUUID(w, r)
		if !ok {
			return
		}
		viewer := auth.FromContext(r.Context())
		meta, err := loadForumPostMeta(r.Context(), deps.Pool, postID)
		if errors.Is(err, pgx.ErrNoRows) {
			forumError(w, http.StatusNotFound, "not_found")
			return
		}
		if err != nil {
			http.Error(w, "load post: "+err.Error(), http.StatusInternalServerError)
			return
		}
		limit, offset := pageParams(r)
		order := "(c.upvotes - c.downvotes) DESC, c.created_at DESC, c.id DESC"
		switch r.URL.Query().Get("sort") {
		case "", "best":
		case "newest":
			order = "c.created_at DESC, c.id DESC"
		default:
			forumError(w, http.StatusBadRequest, "invalid_sort")
			return
		}

		rows, err := deps.Pool.Query(r.Context(), `SELECT `+forumCommentColumns+forumCommentFrom+`
			WHERE c.post_id = $2 AND c.parent_id IS NULL AND c.deleted_at IS NULL
			ORDER BY `+order+`
			LIMIT $3 OFFSET $4`, viewerID(viewer), postID, limit+1, offset)
		if err != nil {
			http.Error(w, "list comments: "+err.Error(), http.StatusInternalServerError)
			return
		}
		top := make([]forumComment, 0, limit)
		for rows.Next() {
			c, err := scanForumComment(rows, viewer, meta.authorID)
			if err != nil {
				rows.Close()
				http.Error(w, "scan comment: "+err.Error(), http.StatusInternalServerError)
				return
			}
			top = append(top, c)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			http.Error(w, "list comments: "+err.Error(), http.StatusInternalServerError)
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
			rows, err := deps.Pool.Query(r.Context(), `SELECT `+forumCommentColumns+forumCommentFrom+`
				WHERE c.parent_id = ANY($2) AND c.deleted_at IS NULL
				ORDER BY c.created_at, c.id`, viewerID(viewer), ids)
			if err != nil {
				http.Error(w, "list replies: "+err.Error(), http.StatusInternalServerError)
				return
			}
			defer rows.Close()
			for rows.Next() {
				c, err := scanForumComment(rows, viewer, meta.authorID)
				if err != nil {
					http.Error(w, "scan reply: "+err.Error(), http.StatusInternalServerError)
					return
				}
				i := index[*c.ParentID]
				top[i].Replies = append(top[i].Replies, c)
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{"comments": top, "has_more": hasMore})
	}
}

type forumCommentRequest struct {
	Body        string     `json:"body"`
	ParentID    *uuid.UUID `json:"parent_id"`
	IsAnonymous bool       `json:"is_anonymous"`
}

// CreateForumComment adds a comment or a reply. Replies attach to top-level
// comments only, so threads stay one level deep.
func CreateForumComment(deps ForumDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		postID, ok := pathUUID(w, r)
		if !ok {
			return
		}
		u, ok := requireUser(w, r)
		if !ok {
			return
		}
		var req forumCommentRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			forumError(w, http.StatusBadRequest, "bad_json")
			return
		}
		body := strings.TrimSpace(req.Body)
		if body == "" || utf8.RuneCountInString(body) > forumCommentMax {
			forumError(w, http.StatusBadRequest, "invalid_body")
			return
		}

		tx, err := deps.Pool.BeginTx(r.Context(), pgx.TxOptions{})
		if err != nil {
			http.Error(w, "tx begin: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer tx.Rollback(r.Context())

		meta, err := loadForumPostMeta(r.Context(), tx, postID)
		if errors.Is(err, pgx.ErrNoRows) {
			forumError(w, http.StatusNotFound, "not_found")
			return
		}
		if err != nil {
			http.Error(w, "load post: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if req.IsAnonymous && !forumAnonSections[meta.section] {
			forumError(w, http.StatusBadRequest, "anonymous_not_allowed")
			return
		}
		if req.ParentID != nil {
			var parentPost uuid.UUID
			var grandparent *uuid.UUID
			err := tx.QueryRow(r.Context(),
				`SELECT post_id, parent_id FROM forum_comments WHERE id = $1 AND deleted_at IS NULL`,
				*req.ParentID).Scan(&parentPost, &grandparent)
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
			http.Error(w, "create comment: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if _, err := tx.Exec(r.Context(),
			`UPDATE forum_posts SET comment_count = comment_count + 1 WHERE id = $1`, postID); err != nil {
			http.Error(w, "bump count: "+err.Error(), http.StatusInternalServerError)
			return
		}
		c, err := scanForumComment(tx.QueryRow(r.Context(),
			`SELECT `+forumCommentColumns+forumCommentFrom+` WHERE c.id = $2`, u.ID, id), u, meta.authorID)
		if err != nil {
			http.Error(w, "load comment: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if err := tx.Commit(r.Context()); err != nil {
			http.Error(w, "tx commit: "+err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusCreated, c)
	}
}

func VoteForumComment(deps ForumDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathUUID(w, r)
		if !ok {
			return
		}
		u, ok := requireUser(w, r)
		if !ok {
			return
		}
		value, ok := decodeVote(w, r)
		if !ok {
			return
		}
		up, down, err := applyForumVote(r.Context(), deps.Pool, "forum_comment_votes", "comment_id", "forum_comments", id, u.ID, value)
		if errors.Is(err, pgx.ErrNoRows) {
			forumError(w, http.StatusNotFound, "not_found")
			return
		}
		if err != nil {
			http.Error(w, "vote: "+err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"upvotes": up, "downvotes": down, "score": up - down, "my_vote": value})
	}
}

// DeleteForumComment soft-deletes a comment (author or admin). Deleting a
// top-level comment also removes its replies, and the post's comment count
// drops by everything that disappeared.
func DeleteForumComment(deps ForumDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathUUID(w, r)
		if !ok {
			return
		}
		u, ok := requireUser(w, r)
		if !ok {
			return
		}
		tx, err := deps.Pool.BeginTx(r.Context(), pgx.TxOptions{})
		if err != nil {
			http.Error(w, "tx begin: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer tx.Rollback(r.Context())

		var postID uuid.UUID
		err = tx.QueryRow(r.Context(), `
			UPDATE forum_comments SET deleted_at = now()
			WHERE id = $1 AND deleted_at IS NULL AND (user_id = $2 OR $3)
			RETURNING post_id`, id, u.ID, u.Role == "admin").Scan(&postID)
		if errors.Is(err, pgx.ErrNoRows) {
			forumError(w, http.StatusNotFound, "not_found")
			return
		}
		if err != nil {
			http.Error(w, "delete comment: "+err.Error(), http.StatusInternalServerError)
			return
		}
		tag, err := tx.Exec(r.Context(),
			`UPDATE forum_comments SET deleted_at = now() WHERE parent_id = $1 AND deleted_at IS NULL`, id)
		if err != nil {
			http.Error(w, "delete replies: "+err.Error(), http.StatusInternalServerError)
			return
		}
		removed := 1 + tag.RowsAffected()
		if _, err := tx.Exec(r.Context(),
			`UPDATE forum_posts SET comment_count = GREATEST(comment_count - $2, 0) WHERE id = $1`,
			postID, removed); err != nil {
			http.Error(w, "update count: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if err := tx.Commit(r.Context()); err != nil {
			http.Error(w, "tx commit: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
