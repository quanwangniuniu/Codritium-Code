package account

import (
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"codritium/backend/internal/auth"
	"codritium/backend/internal/platform/httpx"
)

// maxAvatarBytes caps an upload. The browser resizes to 256px before
// sending, which lands far below this.
const maxAvatarBytes = 256 << 10

// avatarTypes are the image formats accepted, as sniffed from the bytes
// (the request's Content-Type header is not trusted).
var avatarTypes = []string{"image/png", "image/jpeg", "image/webp"}

// putAvatar stores the request body as the caller's avatar and points
// users.avatar_url at the serving endpoint. PUT /api/me/avatar
func (h Handler) putAvatar(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.Require(w, r)
	if !ok {
		return
	}
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxAvatarBytes))
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			httpx.Error(w, http.StatusRequestEntityTooLarge, "avatar_too_large", "Avatar must be at most 256 KB.")
			return
		}
		httpx.BadRequest(w, "bad_avatar", "Could not read the image.")
		return
	}
	contentType := http.DetectContentType(data)
	if !contains(avatarTypes, contentType) {
		httpx.BadRequest(w, "bad_avatar_type", "Avatar must be a PNG, JPEG or WebP image.")
		return
	}

	ctx := r.Context()
	tx, err := h.Pool.Begin(ctx)
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `
		INSERT INTO user_avatars (user_id, content_type, data) VALUES ($1, $2, $3)
		ON CONFLICT (user_id) DO UPDATE
		SET content_type = EXCLUDED.content_type, data = EXCLUDED.data, updated_at = now()`,
		u.ID, contentType, data); err != nil {
		httpx.Internal(w, r, err)
		return
	}
	// The version query changes on every upload, so the image itself can be
	// cached forever.
	next := *u
	next.AvatarURL = fmt.Sprintf("/api/users/%s/avatar?v=%d", u.ID, h.now().UnixMilli())
	if _, err := tx.Exec(ctx, `UPDATE users SET avatar_url = $2 WHERE id = $1`, u.ID, next.AvatarURL); err != nil {
		httpx.Internal(w, r, err)
		return
	}
	if err := tx.Commit(ctx); err != nil {
		httpx.Internal(w, r, err)
		return
	}
	h.writeMeWithDetails(w, r, &next)
}

// deleteAvatar removes the caller's avatar (uploaded or inherited from an
// OAuth provider); the UI then shows the generated default.
// DELETE /api/me/avatar
func (h Handler) deleteAvatar(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.Require(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	tx, err := h.Pool.Begin(ctx)
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `DELETE FROM user_avatars WHERE user_id = $1`, u.ID); err != nil {
		httpx.Internal(w, r, err)
		return
	}
	if _, err := tx.Exec(ctx, `UPDATE users SET avatar_url = NULL WHERE id = $1`, u.ID); err != nil {
		httpx.Internal(w, r, err)
		return
	}
	if err := tx.Commit(ctx); err != nil {
		httpx.Internal(w, r, err)
		return
	}
	next := *u
	next.AvatarURL = ""
	h.writeMeWithDetails(w, r, &next)
}

func (h Handler) writeMeWithDetails(w http.ResponseWriter, r *http.Request, u *auth.User) {
	d, err := loadDetails(r.Context(), h.Pool, u.ID)
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	writeMe(w, u, d)
}

// getAvatar serves a user's uploaded avatar. Public: avatars appear next to
// forum posts and comments. GET /api/users/{id}/avatar
func (h Handler) getAvatar(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpx.NotFound(w, "Avatar not found.")
		return
	}
	var contentType string
	var data []byte
	err = h.Pool.QueryRow(r.Context(),
		`SELECT content_type, data FROM user_avatars WHERE user_id = $1`, id,
	).Scan(&contentType, &data)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.NotFound(w, "Avatar not found.")
		return
	}
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	_, _ = w.Write(data)
}
