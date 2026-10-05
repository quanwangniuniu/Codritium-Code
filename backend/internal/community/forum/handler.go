// Package forum is the community forum: posts, comments, votes, pins.
package forum

import (
	"codritium/backend/internal/auth"
	"codritium/backend/internal/platform/httpx"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Routes registers the forum API. Reads are public (signed-out viewers get
// a nil user); writes require a session, enforced per handler.
func (h Handler) Routes(rt *httpx.Router) {
	rt.Handle("GET /api/forum/posts", h.listForumPosts)
	rt.Handle("GET /api/forum/pinned", h.listPinnedForumPosts)
	rt.Handle("GET /api/forum/trending", h.listTrendingForumPosts)
	rt.Handle("POST /api/forum/posts", h.createForumPost)
	rt.Handle("GET /api/forum/posts/{id}", h.getForumPost)
	rt.Handle("PUT /api/forum/posts/{id}", h.updateForumPost)
	rt.Handle("DELETE /api/forum/posts/{id}", h.deleteForumPost)
	rt.Handle("POST /api/forum/posts/{id}/view", h.recordForumView)
	rt.Handle("POST /api/forum/posts/{id}/vote", h.voteForumPost)
	rt.Handle("POST /api/forum/posts/{id}/pin", h.pinForumPost)
	rt.Handle("GET /api/forum/posts/{id}/comments", h.listForumComments)
	rt.Handle("POST /api/forum/posts/{id}/comments", h.createForumComment)
	rt.Handle("PUT /api/forum/comments/{id}", h.updateForumComment)
	rt.Handle("POST /api/forum/comments/{id}/vote", h.voteForumComment)
	rt.Handle("DELETE /api/forum/comments/{id}", h.deleteForumComment)
}

type Handler struct {
	Pool *pgxpool.Pool
	// Cookie carries the deployment's cookie attributes for the signed-out
	// visitor cookie that view counting uses.
	Cookie auth.CookieOptions
}
