package forum

import (
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"codritium/backend/internal/auth"
	"codritium/backend/internal/platform/httpx"
)

const (
	tagsDefault   = 20
	tagsMax       = 50
	relatedMax    = 5
	myCommentsLen = 200
)

type forumTagCount struct {
	Tag   string `json:"tag"`
	Count int    `json:"count"`
}

// ListForumTags returns tags on live posts with how many posts carry each,
// most used first. ?q= narrows to tags starting with it (editor
// autocomplete); ?limit= caps the list.
func (h Handler) listForumTags(w http.ResponseWriter, r *http.Request) {
	q := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(r.URL.Query().Get("q")), "#"))
	if len(q) > 24 {
		q = q[:24]
	}
	page := httpx.ParsePage(r, tagsDefault, tagsMax, 0)
	rows, err := h.Pool.Query(r.Context(), `
		SELECT t, count(*)::int FROM forum_posts p, unnest(p.tags) AS t
		WHERE p.deleted_at IS NULL AND t LIKE $1 || '%' ESCAPE '\'
		GROUP BY t
		ORDER BY count(*) DESC, t
		LIMIT $2`, escapeLike(q), page.Limit)
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	tags, err := pgx.CollectRows(rows, pgx.RowToStructByPos[forumTagCount])
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"tags": tags})
}

// ListRelatedForumPosts suggests up to five other posts for a post's
// sidebar, scored by shared tags, the same linked problem, and the same
// section; ties go to better-voted, then newer posts.
func (h Handler) listRelatedForumPosts(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.PathUUID(w, r, "id")
	if !ok {
		return
	}
	viewer := auth.FromContext(r.Context())
	rows, err := h.Pool.Query(r.Context(), `
		WITH me AS (
		  SELECT id, tags, section, problem_slug FROM forum_posts WHERE id = $2 AND deleted_at IS NULL
		)
		SELECT `+forumPostColumns("left(p.body_md, 800)")+forumPostFrom+`
		CROSS JOIN me
		WHERE p.deleted_at IS NULL AND p.id <> me.id
		  AND (p.tags && me.tags OR p.problem_slug = me.problem_slug OR p.section = me.section)
		ORDER BY cardinality(ARRAY(SELECT unnest(p.tags) INTERSECT SELECT unnest(me.tags))) * 2
		         + CASE WHEN p.problem_slug = me.problem_slug THEN 3 ELSE 0 END
		         + CASE WHEN p.section = me.section THEN 1 ELSE 0 END DESC,
		         (p.upvotes - p.downvotes) DESC, p.created_at DESC
		LIMIT $3`, viewerID(viewer), id, relatedMax)
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
	if err := rows.Err(); err != nil {
		httpx.Internal(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"posts": posts})
}

type myForumComment struct {
	ID          uuid.UUID `json:"id"`
	PostID      uuid.UUID `json:"post_id"`
	PostTitle   string    `json:"post_title"`
	Excerpt     string    `json:"excerpt"`
	IsAnonymous bool      `json:"is_anonymous"`
	Score       int       `json:"score"`
	CreatedAt   time.Time `json:"created_at"`
}

// ListMyForumComments is the signed-in user's own comments, newest first,
// for their profile. It includes anonymous ones, since only they see it.
func (h Handler) listMyForumComments(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.Require(w, r)
	if !ok {
		return
	}
	page := httpx.ParsePage(r, forumPageDefault, forumPageMax, forumOffsetMax)
	rows, err := h.Pool.Query(r.Context(), `
		SELECT c.id, c.post_id, p.title, left(regexp_replace(c.body, '\s+', ' ', 'g'), $4),
		       c.is_anonymous, c.upvotes - c.downvotes, c.created_at
		FROM forum_comments c
		JOIN forum_posts p ON p.id = c.post_id AND p.deleted_at IS NULL
		WHERE c.user_id = $1 AND c.deleted_at IS NULL
		ORDER BY c.created_at DESC, c.id DESC
		LIMIT $2 OFFSET $3`, u.ID, page.Limit+1, page.Offset, myCommentsLen)
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	comments, err := pgx.CollectRows(rows, pgx.RowToStructByPos[myForumComment])
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	hasMore := len(comments) > page.Limit
	if hasMore {
		comments = comments[:page.Limit]
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"comments": comments, "has_more": hasMore})
}
