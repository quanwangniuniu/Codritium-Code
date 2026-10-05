package forum

import (
	"net/http"
	"strings"
	"unicode/utf8"

	"codritium/backend/internal/auth"
	"codritium/backend/internal/platform/httpx"

	"github.com/google/uuid"
)

const mentionSuggestMax = 8

// setPostFlag turns one of the viewer's per-post toggles (bookmark, follow)
// on or off. Turning it on requires the post to exist; turning it off is
// idempotent. Responds with {key: state}.
func (h Handler) setPostFlag(w http.ResponseWriter, r *http.Request, table, key string) {
	id, ok := httpx.PathUUID(w, r, "id")
	if !ok {
		return
	}
	u, ok := auth.Require(w, r)
	if !ok {
		return
	}
	var req map[string]bool
	if !httpx.Decode(w, r, &req) {
		return
	}
	on, present := req[key]
	if !present {
		httpx.BadRequest(w, "bad_json", "Send {\""+key+"\": true|false}.")
		return
	}
	// table is one of two constants below, never request input.
	if on {
		tag, err := h.Pool.Exec(r.Context(), `
			INSERT INTO `+table+` (post_id, user_id)
			SELECT id, $2 FROM forum_posts WHERE id = $1 AND deleted_at IS NULL
			ON CONFLICT DO NOTHING`, id, u.ID)
		if err != nil {
			httpx.Internal(w, r, err)
			return
		}
		if tag.RowsAffected() == 0 {
			var exists bool
			if err := h.Pool.QueryRow(r.Context(),
				`SELECT EXISTS(SELECT 1 FROM forum_posts WHERE id = $1 AND deleted_at IS NULL)`, id).Scan(&exists); err != nil {
				httpx.Internal(w, r, err)
				return
			}
			if !exists {
				forumError(w, http.StatusNotFound, "not_found")
				return
			}
		}
	} else if _, err := h.Pool.Exec(r.Context(),
		`DELETE FROM `+table+` WHERE post_id = $1 AND user_id = $2`, id, u.ID); err != nil {
		httpx.Internal(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]bool{key: on})
}

func (h Handler) bookmarkForumPost(w http.ResponseWriter, r *http.Request) {
	h.setPostFlag(w, r, "forum_bookmarks", "bookmarked")
}

func (h Handler) followForumPost(w http.ResponseWriter, r *http.Request) {
	h.setPostFlag(w, r, "forum_post_follows", "following")
}

// ListForumBookmarks returns the viewer's saved posts, most recently saved
// first.
func (h Handler) listForumBookmarks(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.Require(w, r)
	if !ok {
		return
	}
	page := httpx.ParsePage(r, forumPageDefault, forumPageMax, forumOffsetMax)
	rows, err := h.Pool.Query(r.Context(), `SELECT `+forumPostColumns("left(p.body_md, 800)")+forumPostFrom+`
		JOIN forum_bookmarks b ON b.post_id = p.id AND b.user_id = $1
		WHERE p.deleted_at IS NULL
		ORDER BY b.created_at DESC, p.id DESC
		LIMIT $2 OFFSET $3`, u.ID, page.Limit+1, page.Offset)
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	defer rows.Close()
	posts := make([]forumPost, 0, page.Limit)
	for rows.Next() {
		p, err := scanForumPost(rows, u, false)
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
	hasMore := len(posts) > page.Limit
	if hasMore {
		posts = posts[:page.Limit]
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"posts": posts, "has_more": hasMore})
}

type mentionSuggestion struct {
	ID          uuid.UUID `json:"id"`
	Handle      string    `json:"handle"`
	DisplayName string    `json:"display_name"`
	AvatarURL   string    `json:"avatar_url"`
	AvatarColor string    `json:"avatar_color"`
}

// SuggestMentions powers @-autocomplete: users whose handle or display name
// starts with ?q. With ?post_id, people already in that thread (under their
// own name, never anonymously) come first, and an empty ?q lists only them,
// so the endpoint can't page through every user. Signed-in only.
func (h Handler) suggestMentions(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.Require(w, r)
	if !ok {
		return
	}
	q := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(r.URL.Query().Get("q")), "@"))
	if utf8.RuneCountInString(q) > 40 {
		q = string([]rune(q)[:40])
	}
	postID := uuid.Nil
	if s := r.URL.Query().Get("post_id"); s != "" {
		if id, err := uuid.Parse(s); err == nil {
			postID = id
		}
	}
	rows, err := h.Pool.Query(r.Context(), `
		WITH thread AS (
		  SELECT user_id FROM forum_posts WHERE id = $2 AND deleted_at IS NULL AND NOT is_anonymous
		  UNION
		  SELECT user_id FROM forum_comments WHERE post_id = $2 AND deleted_at IS NULL AND NOT is_anonymous
		)
		SELECT u.id, u.handle, u.display_name, COALESCE(u.avatar_url,''), COALESCE(u.avatar_color,'')
		FROM users u
		WHERE u.id <> $3
		  AND CASE WHEN $1 = '' THEN u.id IN (SELECT user_id FROM thread)
		           ELSE lower(u.handle) LIKE $1 || '%' ESCAPE '\' OR lower(u.display_name) LIKE $1 || '%' ESCAPE '\'
		      END
		ORDER BY (u.id IN (SELECT user_id FROM thread)) DESC,
		         (lower(u.handle) LIKE $1 || '%' ESCAPE '\') DESC,
		         lower(u.handle)
		LIMIT $4`, escapeLike(q), postID, u.ID, mentionSuggestMax)
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	defer rows.Close()
	out := []mentionSuggestion{}
	for rows.Next() {
		var s mentionSuggestion
		if err := rows.Scan(&s.ID, &s.Handle, &s.DisplayName, &s.AvatarURL, &s.AvatarColor); err != nil {
			httpx.Internal(w, r, err)
			return
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		httpx.Internal(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"users": out})
}
