package handlers

import (
	"encoding/json"
	"net/http"

	"codritium/backend/internal/auth"
	"codritium/backend/internal/llm"
)

type DecisionDeps struct {
	Waiter *llm.DecisionWaiter
}

type decisionRequest struct {
	ToolUseID     string `json:"tool_use_id"`
	Decision      string `json:"decision"` // approve | reject | modify
	ModifiedInput string `json:"modified_input,omitempty"`
	Reason        string `json:"reason,omitempty"`
	Comment       string `json:"comment,omitempty"`
}

// PostDecision routes the candidate's verdict on a pending ToolUseProposed
// to the chat goroutine via DecisionWaiter.Notify.
//
// Responses:
//   204 — pending tool_use_id was found and notified
//   400 — invalid JSON, missing tool_use_id, or decision not in
//         {approve, reject, modify}
//   401 — no auth cookie
//   404 — no pending Wait for that tool_use_id (already notified, expired,
//         or never registered)
func PostDecision(deps DecisionDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.FromContext(r.Context())
		if u == nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		var req decisionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad json: "+err.Error(), http.StatusBadRequest)
			return
		}
		if req.ToolUseID == "" {
			http.Error(w, "tool_use_id required", http.StatusBadRequest)
			return
		}
		d := llm.Decision{
			Kind:          req.Decision,
			ModifiedInput: req.ModifiedInput,
			Reason:        req.Reason,
			Comment:       req.Comment,
		}
		if !d.IsValid() {
			http.Error(w, "decision must be approve|reject|modify", http.StatusBadRequest)
			return
		}
		if !deps.Waiter.Notify(req.ToolUseID, d) {
			http.Error(w, "no pending tool_use", http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
