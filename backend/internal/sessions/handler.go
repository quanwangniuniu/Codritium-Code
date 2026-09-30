package sessions

import (
	"errors"
	"net/http"

	"codritium/backend/internal/auth"
	"codritium/backend/internal/platform/httpx"
	"codritium/backend/internal/problems"
)

// Handler serves session start/resume and the attempted list.
type Handler struct {
	Store    Store
	Problems problems.Store
}

func (h Handler) Routes(rt *httpx.Router) {
	rt.Handle("POST /api/sessions", h.start)
	rt.Handle("GET /api/me/attempted", h.attempted)
}

type startRequest struct {
	ChallengeSlug string `json:"challenge_slug"`
	Difficulty    string `json:"difficulty,omitempty"` // ignored; the problem decides
	ForceNew      bool   `json:"force_new"`
}

type startResponse struct {
	SessionID   string `json:"session_id"`
	Created     bool   `json:"created"`
	StartedAt   string `json:"started_at"`
	ChallengeID string `json:"challenge_id"`
}

// start resumes the caller's unsubmitted session for the problem from the
// last ReuseWindow, or creates one (always, with force_new). `created`
// tells the frontend which happened.
func (h Handler) start(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.Require(w, r)
	if !ok {
		return
	}
	var req startRequest
	if !httpx.Decode(w, r, &req) {
		return
	}
	if req.ChallengeSlug == "" {
		httpx.BadRequest(w, "challenge_slug_required", "challenge_slug is required.")
		return
	}
	p, err := h.Problems.Get(r.Context(), req.ChallengeSlug)
	if errors.Is(err, problems.ErrNotFound) {
		httpx.NotFound(w, "Problem not found.")
		return
	}
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}

	if !req.ForceNew {
		id, startedAt, found, err := h.Store.FindReusable(r.Context(), u.ID, p.Slug)
		if err != nil {
			httpx.Internal(w, r, err)
			return
		}
		if found {
			httpx.JSON(w, http.StatusOK, startResponse{SessionID: id.String(), StartedAt: startedAt, ChallengeID: p.Slug})
			return
		}
	}
	id, startedAt, err := h.Store.Create(r.Context(), u.ID, p.Slug, p.Difficulty)
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, startResponse{SessionID: id.String(), Created: true, StartedAt: startedAt, ChallengeID: p.Slug})
}

// attempted lists problems the caller has opened, most recent first.
func (h Handler) attempted(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.Require(w, r)
	if !ok {
		return
	}
	out, err := h.Store.Attempted(r.Context(), u.ID)
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}
