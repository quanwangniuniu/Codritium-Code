package agent

import (
	"encoding/json"
	"fmt"
	"net/http"

	"codritium/backend/internal/platform/httpx"
	"codritium/backend/internal/sessions"

	"github.com/google/uuid"

	"codritium/backend/internal/auth"
	"codritium/backend/internal/events"
)

// SessionStream is GET /api/sessions/{id}/stream. It is a long-lived SSE
// connection that pushes two SSE event types (chat_loop_skeleton §6):
//
//	event: agent_event   ← 16 structured event kinds (closed set)
//	event: message       ← assistant streaming text deltas (free text)
//
// Frontend subscribes once per session; one EventSource handles both
// streams. Spec §5 keeps text off session_events; the text path goes
// through TextBroadcaster, not events.Store.
//
// Auth: caller must own the session (candidate_id == handle).
func (h Handler) sessionStream(w http.ResponseWriter, r *http.Request) {
	u := auth.FromContext(r.Context())
	if u == nil {
		httpx.Unauthorized(w)
		return
	}
	sessionID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "Session id must be a uuid.")
		return
	}

	owned, err := (sessions.Store{Pool: h.Pool}).SessionOwnedBy(r.Context(), sessionID, u)
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	if !owned {
		httpx.Error(w, http.StatusNotFound, "not_found", "Session not found.")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		httpx.Error(w, http.StatusInternalServerError, "internal", "Streaming unsupported.")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-store")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	// Subscribe to both fan-out sources.
	eventsCh := make(chan events.Envelope, 32)
	cancelEvents := h.Events.Subscribe(sessionID, eventsCh)
	defer cancelEvents()

	textCh := make(chan string, 64)
	var cancelText func()
	if h.Text != nil {
		cancelText = h.Text.Subscribe(sessionID, textCh)
		defer cancelText()
	}

	// Send a comment to flush headers immediately.
	_, _ = w.Write([]byte(": connected\n\n"))
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case env := <-eventsCh:
			if err := writeSSE(w, "agent_event", env); err != nil {
				return
			}
			flusher.Flush()
		case text := <-textCh:
			if err := writeSSE(w, "message", map[string]string{"text": text}); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func writeSSE(w http.ResponseWriter, eventType string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventType, data)
	return err
}
