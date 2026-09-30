// Package account serves the signed-in user's own record: GET and
// PATCH /api/me.
package account

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5/pgxpool"

	"codritium/backend/internal/platform/httpx"

	"codritium/backend/internal/auth"
)

// WriteUserJSON writes the current user as JSON (used by /api/me).
// is_pro and avatar_color stay in the payload so older frontend code keeps
// working while it migrates to tier and avatar_url.
func WriteUserJSON(w http.ResponseWriter, u *auth.User) {
	if u == nil {
		httpx.JSON(w, http.StatusUnauthorized, map[string]any{"error": "no user"})
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{
		"id":           u.ID,
		"handle":       u.Handle,
		"email":        u.Email,
		"display_name": u.DisplayName,
		"region":       u.Region,
		"tier":         u.Tier,
		"credits":      u.Credits,
		"role":         u.Role,
		"avatar_url":   u.AvatarURL,
		"avatar_color": u.AvatarColor,
		"bio":          u.Bio,
		"is_pro":       u.IsPro,
	})
}

// Routes registers the account API.
func (h Handler) Routes(rt *httpx.Router) {
	rt.Handle("GET /api/me", h.me)
	rt.Handle("PATCH /api/me", h.updateMe)
}

// Handler serves /api/me.
type Handler struct {
	Pool *pgxpool.Pool
}

func (h Handler) me(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.Require(w, r)
	if !ok {
		return
	}
	WriteUserJSON(w, u)
}

// Profile field limits (in characters).
const (
	maxDisplayNameRunes = 50
	maxBioRunes         = 280
	maxRegionRunes      = 40
)

type updateMeRequest struct {
	DisplayName *string `json:"display_name"`
	Bio         *string `json:"bio"`
	Region      *string `json:"region"`
}

// UpdateMe edits the signed-in user's display name, bio, and region.
// Omitted fields are left unchanged. PATCH /api/me
func (h Handler) updateMe(w http.ResponseWriter, r *http.Request) {
	u := auth.FromContext(r.Context())
	if u == nil {
		httpx.JSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 8<<10)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	var req updateMeRequest
	if err := dec.Decode(&req); err != nil {
		httpx.JSON(w, http.StatusBadRequest, map[string]any{"error": "bad_json"})
		return
	}
	next := *u
	if err := applyProfileUpdate(&next, req); err != nil {
		httpx.JSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	_, err := h.Pool.Exec(r.Context(), `
		UPDATE users SET display_name = $2, bio = NULLIF($3, ''), region = NULLIF($4, '')
		WHERE id = $1`, u.ID, next.DisplayName, next.Bio, next.Region)
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	WriteUserJSON(w, &next)
}

func applyProfileUpdate(u *auth.User, req updateMeRequest) error {
	if req.DisplayName != nil {
		v := strings.TrimSpace(*req.DisplayName)
		if v == "" || utf8.RuneCountInString(v) > maxDisplayNameRunes {
			return errors.New("invalid_display_name")
		}
		u.DisplayName = v
	}
	if req.Bio != nil {
		v := strings.TrimSpace(*req.Bio)
		if utf8.RuneCountInString(v) > maxBioRunes {
			return errors.New("invalid_bio")
		}
		u.Bio = v
	}
	if req.Region != nil {
		v := strings.TrimSpace(*req.Region)
		if utf8.RuneCountInString(v) > maxRegionRunes {
			return errors.New("invalid_region")
		}
		u.Region = v
	}
	return nil
}
