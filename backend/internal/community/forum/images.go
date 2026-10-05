package forum

import (
	"errors"
	"io"
	"net/http"
	"slices"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"codritium/backend/internal/auth"
	"codritium/backend/internal/platform/httpx"
)

// maxImageBytes caps one upload. The editor resizes to 1600px before
// sending, which typically lands well under this.
const maxImageBytes = 1 << 20

// imageTypes are accepted as sniffed from the bytes; the request's
// Content-Type header isn't trusted.
var imageTypes = []string{"image/png", "image/jpeg", "image/webp", "image/gif"}

// UploadForumImage stores the request body as an image for a post or
// comment and returns its URL for the markdown. POST /api/forum/images
func (h Handler) uploadForumImage(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.Require(w, r)
	if !ok || !h.allow(w, r, u, imageLimit, "") {
		return
	}
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxImageBytes))
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			httpx.Error(w, http.StatusRequestEntityTooLarge, "image_too_large", "Images must be at most 1 MB.")
			return
		}
		httpx.BadRequest(w, "bad_image", "Could not read the image.")
		return
	}
	contentType := http.DetectContentType(data)
	if !slices.Contains(imageTypes, contentType) {
		httpx.BadRequest(w, "bad_image_type", "Images must be PNG, JPEG, WebP, or GIF.")
		return
	}
	var id uuid.UUID
	if err := h.Pool.QueryRow(r.Context(),
		`INSERT INTO forum_images (user_id, content_type, data) VALUES ($1, $2, $3) RETURNING id`,
		u.ID, contentType, data).Scan(&id); err != nil {
		httpx.Internal(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]string{"id": id.String(), "url": "/api/forum/images/" + id.String()})
}

// GetForumImage serves an uploaded image. Public, and immutable: an id
// always names the same bytes. GET /api/forum/images/{id}
func (h Handler) getForumImage(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpx.NotFound(w, "Image not found.")
		return
	}
	var contentType string
	var data []byte
	err = h.Pool.QueryRow(r.Context(),
		`SELECT content_type, data FROM forum_images WHERE id = $1`, id).Scan(&contentType, &data)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.NotFound(w, "Image not found.")
		return
	}
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	// Served bytes are only ever shown as an image; never run anything.
	w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	_, _ = w.Write(data)
}
