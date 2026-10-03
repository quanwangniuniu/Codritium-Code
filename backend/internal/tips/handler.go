package tips

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"codritium/backend/internal/platform/httpx"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"codritium/backend/internal/auth"
)

// Handler holds the dependencies for the tips endpoints. The Agent
// talks to Gemini; the Pool reads candidate_sessions joined to problems
// and reads/writes tips_messages. There is intentionally no chat_v2 /
// decision_waiter / event store reference — tips-agent is hard-isolated
// from the candidate coding partner.
// Routes registers the tips (Socratic tutor) API.
func (h Handler) Routes(rt *httpx.Router) {
	rt.Handle("POST /api/tips", h.postTips)
	rt.Handle("GET /api/tips/messages", h.getTipsMessages)
}

// Handler serves the tips API. Agent is nil when no Gemini key is set.
type Handler struct {
	Pool   *pgxpool.Pool
	Agent  *Agent
	Filter *MultiTurnFilter
}

type tipsRequest struct {
	SessionID    string            `json:"session_id"`
	Message      string            `json:"message"`
	FileContents map[string]string `json:"file_contents,omitempty"`
}

type tipsMessagesResponse struct {
	Messages []tipsMessagePayload `json:"messages"`
}

type tipsMessagePayload struct {
	Seq       int    `json:"seq"`
	Role      string `json:"role"`
	Text      string `json:"text"`
	CreatedAt string `json:"created_at"`
}

// PostTips streams one tutor turn over SSE. The contract:
//   - mode must be practice (simulator / reply hard-fail).
//   - session must not be submitted.
//   - the candidate must own the session.
//   - tutor responses are persisted to tips_messages, NEVER to session_events.
//
// Stream events:
//
//	data: {"delta":"…"}     repeated, one per chunk
//	data: {"done":true}     terminal on success
//	data: {"error":"…"}     terminal on failure
//
// Both filter refusals and model responses are persisted with role=model.
func (h Handler) postTips(w http.ResponseWriter, r *http.Request) {
	u := auth.FromContext(r.Context())
	if u == nil {
		httpx.Unauthorized(w)
		return
	}
	if h.Agent == nil {
		httpx.Error(w, http.StatusServiceUnavailable, "unavailable", "Tips agent disabled.")
		return
	}

	var req tipsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "Bad json.")
		return
	}
	sessionID, ok := resolveTipsSession(w, req.SessionID)
	if !ok {
		return
	}
	message := strings.TrimSpace(req.Message)
	if message == "" {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "Message required.")
		return
	}

	sc, err := loadAndAuthorizeTipsSession(r, w, h, sessionID, u.ID)
	if err != nil {
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		httpx.Error(w, http.StatusInternalServerError, "internal", "Streaming unsupported.")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	// Persist the user turn before any model work, so a stream
	// interruption still leaves a complete user message in history.
	if _, err := AppendTipsTurn(r.Context(), h.Pool, sessionID, "user", message); err != nil {
		writeSSEError(w, flusher, "persist user turn: "+err.Error())
		return
	}

	history, err := LoadTipsHistory(r.Context(), h.Pool, sessionID)
	if err != nil {
		writeSSEError(w, flusher, "load history: "+err.Error())
		return
	}
	turns := historyToTurns(history)

	// Filter the conversation BEFORE invoking the model. A regex hit
	// (or a YES classifier verdict) substitutes a canned Socratic
	// redirect for the model response and persists it as the model
	// turn so the candidate sees a continuous conversation.
	if h.Filter != nil {
		verdict := h.Filter.Check(r.Context(), turns)
		if !verdict.Allow {
			const refusal = "I'm here to help you think through this problem. Let's focus on what you've tried so far — what's the smallest piece you'd like to talk through?"
			if _, err := AppendTipsTurn(r.Context(), h.Pool, sessionID, "model", refusal); err != nil {
				writeSSEError(w, flusher, "persist refusal: "+err.Error())
				return
			}
			writeSSEDelta(w, flusher, refusal)
			writeSSEDone(w, flusher)
			return
		}
	}

	// Best-effort summary of the chat_v2 session so the tutor sees rough
	// state of what the candidate has done with the AI agent. A SQL
	// failure isn't fatal — an empty summary just drops the section.
	agentSummary, _ := SummarizeSessionEvents(r.Context(), h.Pool, sessionID)

	stream, err := h.Agent.AskStream(r.Context(), Request{
		Mode:         ModePractice,
		SoulPrebake:  sc.Problem.SoulPrebake,
		AgentSummary: agentSummary,
		Conversation: turns,
		FileContents: req.FileContents,
	})
	if err != nil {
		writeSSEError(w, flusher, "tips agent: "+err.Error())
		return
	}

	var modelText strings.Builder
	for chunk := range stream {
		if chunk.Err != nil {
			writeSSEError(w, flusher, chunk.Err.Error())
			return
		}
		if chunk.Done {
			break
		}
		modelText.WriteString(chunk.Delta)
		writeSSEDelta(w, flusher, chunk.Delta)
	}

	if text := modelText.String(); strings.TrimSpace(text) != "" {
		if _, err := AppendTipsTurn(r.Context(), h.Pool, sessionID, "model", text); err != nil {
			writeSSEError(w, flusher, "persist model turn: "+err.Error())
			return
		}
	}
	writeSSEDone(w, flusher)
}

// GetTipsMessages returns the persisted tutor conversation for a session.
// The candidate must own the session; mode must be practice. Used on
// session resume so the tab can re-render the full history.
func (h Handler) getTipsMessages(w http.ResponseWriter, r *http.Request) {
	u := auth.FromContext(r.Context())
	if u == nil {
		httpx.Unauthorized(w)
		return
	}
	raw := r.URL.Query().Get("session_id")
	sessionID, ok := resolveTipsSession(w, raw)
	if !ok {
		return
	}
	if _, err := loadAndAuthorizeTipsSession(r, w, h, sessionID, u.ID); err != nil {
		return
	}
	history, err := LoadTipsHistory(r.Context(), h.Pool, sessionID)
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	out := tipsMessagesResponse{Messages: make([]tipsMessagePayload, 0, len(history))}
	for _, t := range history {
		out.Messages = append(out.Messages, tipsMessagePayload{
			Seq:       t.Seq,
			Role:      t.Role,
			Text:      t.Text,
			CreatedAt: t.CreatedAt,
		})
	}
	httpx.JSON(w, http.StatusOK, out)
}

func resolveTipsSession(w http.ResponseWriter, raw string) (uuid.UUID, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "Session_id required.")
		return uuid.UUID{}, false
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "Session_id must be a UUID.")
		return uuid.UUID{}, false
	}
	return id, true
}

func loadAndAuthorizeTipsSession(r *http.Request, w http.ResponseWriter, h Handler, sessionID uuid.UUID, candidate uuid.UUID) (SessionContext, error) {
	sc, err := LoadSessionContext(r.Context(), h.Pool, sessionID)
	if errors.Is(err, ErrSessionNotFound) {
		httpx.Error(w, http.StatusNotFound, "not_found", "Session not found.")
		return SessionContext{}, err
	}
	if err != nil {
		httpx.Internal(w, r, err)
		return SessionContext{}, err
	}
	if sc.UserID != candidate {
		// Same answer as a missing session: never reveal someone else's.
		httpx.Error(w, http.StatusNotFound, "not_found", "Session not found.")
		return SessionContext{}, fmt.Errorf("ownership mismatch")
	}
	if sc.Mode != ModePractice {
		http.Error(w, "tips disabled for mode: "+string(sc.Mode), http.StatusForbidden)
		return SessionContext{}, fmt.Errorf("mode not practice")
	}
	if sc.Submitted {
		httpx.Error(w, http.StatusForbidden, "forbidden", "Session already submitted.")
		return SessionContext{}, fmt.Errorf("session submitted")
	}
	return sc, nil
}

func historyToTurns(rows []StoredTurn) []Turn {
	out := make([]Turn, 0, len(rows))
	for _, t := range rows {
		text := strings.TrimSpace(t.Text)
		if text == "" {
			continue
		}
		out = append(out, Turn{Role: t.Role, Text: text})
	}
	return out
}

func writeSSEDelta(w http.ResponseWriter, flusher http.Flusher, delta string) {
	payload, _ := json.Marshal(struct {
		Delta string `json:"delta"`
	}{Delta: delta})
	_, _ = fmt.Fprintf(w, "data: %s\n\n", payload)
	flusher.Flush()
}

func writeSSEDone(w http.ResponseWriter, flusher http.Flusher) {
	_, _ = fmt.Fprint(w, "data: {\"done\":true}\n\n")
	flusher.Flush()
}

func writeSSEError(w http.ResponseWriter, flusher http.Flusher, msg string) {
	payload, _ := json.Marshal(struct {
		Error string `json:"error"`
	}{Error: msg})
	_, _ = fmt.Fprintf(w, "data: %s\n\n", payload)
	flusher.Flush()
}
