package handlers

import (
	"net/http"

	"codritium/backend/internal/auth"
)

// WriteUserJSON writes the current user as JSON (used by /api/me).
// is_pro and avatar_color stay in the payload so older frontend code keeps
// working while it migrates to tier and avatar_url.
func WriteUserJSON(w http.ResponseWriter, u *auth.User) {
	if u == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "no user"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
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
