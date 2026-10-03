package agent

import (
	"context"
	"encoding/json"
	"net/http"

	"codritium/backend/internal/platform/httpx"

	"github.com/google/uuid"

	"codritium/backend/internal/auth"
	"codritium/backend/internal/llm"
)

// SessionOwner answers whether a candidate session belongs to a user.
type SessionOwner interface {
	SessionOwnedBy(ctx context.Context, sessionID uuid.UUID, u *auth.User) (bool, error)
}

type decisionRequest struct {
	SessionID     string `json:"session_id"`
	ToolUseID     string `json:"tool_use_id"`
	Decision      string `json:"decision"` // approve | reject | modify
	ModifiedInput string `json:"modified_input,omitempty"`
	Reason        string `json:"reason,omitempty"`
	Comment       string `json:"comment,omitempty"`
}

// PostDecision routes the candidate's verdict on a pending ToolUseProposed
// to the chat goroutine via DecisionWaiter.Notify. Pending decisions are
// keyed by (session, tool_use_id) and only the session's owner may decide.
//
// Responses:
//
//	204 — pending decision found and notified
//	400 — invalid JSON, bad session_id, missing tool_use_id, or decision
//	      not in {approve, reject, modify}
//	401 — no auth cookie
//	404 — session not owned by caller, or no pending Wait for that id
func (h Handler) postDecision(w http.ResponseWriter, r *http.Request) {
	u := auth.FromContext(r.Context())
	if u == nil {
		httpx.Unauthorized(w)
		return
	}
	var req decisionRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "Bad json.")
		return
	}
	sessionID, err := uuid.Parse(req.SessionID)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "Session_id required.")
		return
	}
	if req.ToolUseID == "" {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "Tool_use_id required.")
		return
	}
	d := llm.Decision{
		Kind:          req.Decision,
		ModifiedInput: req.ModifiedInput,
		Reason:        req.Reason,
		Comment:       req.Comment,
	}
	if !d.IsValid() {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "Decision must be approve|reject|modify.")
		return
	}
	owned, err := h.Sessions.SessionOwnedBy(r.Context(), sessionID, u)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "Internal error.")
		return
	}
	// Someone else's session looks exactly like a missing one.
	if !owned || !h.Waiter.Notify(llm.DecisionKey(sessionID, req.ToolUseID), d) {
		httpx.Error(w, http.StatusNotFound, "not_found", "No pending tool_use.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
