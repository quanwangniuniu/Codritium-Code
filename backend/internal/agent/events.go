package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"codritium/backend/internal/platform/httpx"
	"codritium/backend/internal/sessions"

	"github.com/google/uuid"

	"codritium/backend/internal/auth"
	"codritium/backend/internal/events"
)

type postEventRequest struct {
	SessionID string         `json:"session_id"`
	Kind      string         `json:"kind"`
	Payload   map[string]any `json:"payload"`
}

// frontendEmittableKinds is the closed set of kinds the candidate's
// browser is allowed to push directly. Backend-emitted kinds
// (tool_use_proposed, candidate_approved, …) must not appear here —
// they're authoritative inside the chat goroutine.
var frontendEmittableKinds = map[string]struct{}{
	"ai_output_read":          {},
	"candidate_reverted_edit": {},
}

// PostEvent persists a frontend-originating event after verifying the
// caller owns the session.
//
// Responses:
//
//	204 — accepted and persisted
//	400 — bad JSON / unknown or backend-only kind / payload decode fails
//	401 — no auth cookie
//	403 — caller is not the candidate on that session
//	404 — session_id not found
func (h Handler) postEvent(w http.ResponseWriter, r *http.Request) {
	u := auth.FromContext(r.Context())
	if u == nil {
		httpx.Unauthorized(w)
		return
	}
	var req postEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "Bad json.")
		return
	}
	sessionID, err := uuid.Parse(req.SessionID)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "Session_id must be a uuid.")
		return
	}
	if _, ok := frontendEmittableKinds[req.Kind]; !ok {
		httpx.BadRequest(w, "kind_not_allowed", "That event kind cannot be sent from the browser.")
		return
	}
	owned, err := (sessions.Store{Pool: h.Pool}).SessionOwnedBy(r.Context(), sessionID, u)
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	if !owned {
		// Don't leak whether the session exists vs belongs to someone else.
		httpx.Error(w, http.StatusNotFound, "not_found", "Session not found.")
		return
	}

	// Construct the right concrete Event so the store still rejects
	// arbitrary payloads at the type boundary.
	var ev events.Event
	switch req.Kind {
	case "ai_output_read":
		var p events.AIOutputRead
		if err := decodeInto(req.Payload, &p); err != nil {
			httpx.BadRequest(w, "invalid_payload", "Payload does not match the event kind.")
			return
		}
		ev = p
	case "candidate_reverted_edit":
		var p events.CandidateRevertedEdit
		if err := decodeInto(req.Payload, &p); err != nil {
			httpx.BadRequest(w, "invalid_payload", "Payload does not match the event kind.")
			return
		}
		ev = p
	}
	if _, err := h.Events.Append(r.Context(), sessionID, ev); err != nil {
		httpx.Internal(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// decodeInto re-encodes a generic map through JSON into the typed event
// struct with DisallowUnknownFields so the frontend can't smuggle
// unrecognized keys past the type boundary.
func decodeInto(in map[string]any, out any) error {
	if in == nil {
		return fmt.Errorf("payload required")
	}
	b, err := json.Marshal(in)
	if err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(out); err != nil {
		return err
	}
	return nil
}
