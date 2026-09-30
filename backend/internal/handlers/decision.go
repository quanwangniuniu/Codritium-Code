package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"codritium/backend/internal/auth"
	"codritium/backend/internal/llm"
)

// SessionOwner answers whether a candidate session belongs to a user.
type SessionOwner interface {
	SessionOwnedBy(ctx context.Context, sessionID uuid.UUID, u *auth.User) (bool, error)
}

// PGSessionOwner checks ownership against candidate_sessions, which keys
// candidates by handle (see migration 0002).
type PGSessionOwner struct{ Pool *pgxpool.Pool }

func (o PGSessionOwner) SessionOwnedBy(ctx context.Context, sessionID uuid.UUID, u *auth.User) (bool, error) {
	var candidate string
	err := o.Pool.QueryRow(ctx,
		`SELECT candidate_id FROM candidate_sessions WHERE session_id = $1`, sessionID,
	).Scan(&candidate)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return candidate == u.Handle, nil
}

type DecisionDeps struct {
	Waiter   *llm.DecisionWaiter
	Sessions SessionOwner
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
func PostDecision(deps DecisionDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.FromContext(r.Context())
		if u == nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		var req decisionRequest
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		sessionID, err := uuid.Parse(req.SessionID)
		if err != nil {
			http.Error(w, "session_id required", http.StatusBadRequest)
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
		owned, err := deps.Sessions.SessionOwnedBy(r.Context(), sessionID, u)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		// Someone else's session looks exactly like a missing one.
		if !owned || !deps.Waiter.Notify(llm.DecisionKey(sessionID, req.ToolUseID), d) {
			http.Error(w, "no pending tool_use", http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
