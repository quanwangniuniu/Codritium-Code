package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"codritium/backend/internal/auth"
	"codritium/backend/internal/tips"
)

// TipsDeps is the dependency bundle for the tips endpoints. The Agent
// talks to Gemini; the Pool reads candidate_sessions joined to problems
// and reads/writes tips_messages. There is intentionally no chat_v2 /
// decision_waiter / event store reference — tips-agent is hard-isolated
// from the candidate coding partner.
type TipsDeps struct {
	Pool   *pgxpool.Pool
	Agent  *tips.Agent
	Filter *tips.MultiTurnFilter
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
func PostTips(deps TipsDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.FromContext(r.Context())
		if u == nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if deps.Agent == nil {
			http.Error(w, "tips agent disabled", http.StatusServiceUnavailable)
			return
		}

		var req tipsRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad json: "+err.Error(), http.StatusBadRequest)
			return
		}
		sessionID, ok := resolveTipsSession(w, req.SessionID)
		if !ok {
			return
		}
		message := strings.TrimSpace(req.Message)
		if message == "" {
			http.Error(w, "message required", http.StatusBadRequest)
			return
		}

		sc, err := loadAndAuthorizeTipsSession(r, w, deps, sessionID, u.Handle)
		if err != nil {
			return
		}

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")
		w.WriteHeader(http.StatusOK)

		// Persist the user turn before any model work, so a stream
		// interruption still leaves a complete user message in history.
		if _, err := tips.AppendTipsTurn(r.Context(), deps.Pool, sessionID, "user", message); err != nil {
			writeSSEError(w, flusher, "persist user turn: "+err.Error())
			return
		}

		history, err := tips.LoadTipsHistory(r.Context(), deps.Pool, sessionID)
		if err != nil {
			writeSSEError(w, flusher, "load history: "+err.Error())
			return
		}
		turns := historyToTurns(history)

		// Filter the conversation BEFORE invoking the model. A regex hit
		// (or a YES classifier verdict) substitutes a canned Socratic
		// redirect for the model response and persists it as the model
		// turn so the candidate sees a continuous conversation.
		if deps.Filter != nil {
			verdict := deps.Filter.Check(r.Context(), turns)
			if !verdict.Allow {
				const refusal = "I'm here to help you think through this problem. Let's focus on what you've tried so far — what's the smallest piece you'd like to talk through?"
				if _, err := tips.AppendTipsTurn(r.Context(), deps.Pool, sessionID, "model", refusal); err != nil {
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
		agentSummary, _ := tips.SummarizeSessionEvents(r.Context(), deps.Pool, sessionID)

		stream, err := deps.Agent.AskStream(r.Context(), tips.Request{
			Mode:         tips.ModePractice,
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
			if _, err := tips.AppendTipsTurn(r.Context(), deps.Pool, sessionID, "model", text); err != nil {
				writeSSEError(w, flusher, "persist model turn: "+err.Error())
				return
			}
		}
		writeSSEDone(w, flusher)
	}
}

// GetTipsMessages returns the persisted tutor conversation for a session.
// The candidate must own the session; mode must be practice. Used on
// session resume so the tab can re-render the full history.
func GetTipsMessages(deps TipsDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.FromContext(r.Context())
		if u == nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		raw := r.URL.Query().Get("session_id")
		sessionID, ok := resolveTipsSession(w, raw)
		if !ok {
			return
		}
		if _, err := loadAndAuthorizeTipsSession(r, w, deps, sessionID, u.Handle); err != nil {
			return
		}
		history, err := tips.LoadTipsHistory(r.Context(), deps.Pool, sessionID)
		if err != nil {
			http.Error(w, "load history: "+err.Error(), http.StatusInternalServerError)
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
		writeJSON(w, http.StatusOK, out)
	}
}

func resolveTipsSession(w http.ResponseWriter, raw string) (uuid.UUID, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		http.Error(w, "session_id required", http.StatusBadRequest)
		return uuid.UUID{}, false
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		http.Error(w, "session_id must be a UUID", http.StatusBadRequest)
		return uuid.UUID{}, false
	}
	return id, true
}

func loadAndAuthorizeTipsSession(r *http.Request, w http.ResponseWriter, deps TipsDeps, sessionID uuid.UUID, candidate string) (tips.SessionContext, error) {
	sc, err := tips.LoadSessionContext(r.Context(), deps.Pool, sessionID)
	if errors.Is(err, tips.ErrSessionNotFound) {
		http.Error(w, "session not found", http.StatusNotFound)
		return tips.SessionContext{}, err
	}
	if err != nil {
		http.Error(w, "load session: "+err.Error(), http.StatusInternalServerError)
		return tips.SessionContext{}, err
	}
	if sc.CandidateID != candidate {
		http.Error(w, "session belongs to another candidate", http.StatusForbidden)
		return tips.SessionContext{}, fmt.Errorf("ownership mismatch")
	}
	if sc.Mode != tips.ModePractice {
		http.Error(w, "tips disabled for mode: "+string(sc.Mode), http.StatusForbidden)
		return tips.SessionContext{}, fmt.Errorf("mode not practice")
	}
	if sc.Submitted {
		http.Error(w, "session already submitted", http.StatusForbidden)
		return tips.SessionContext{}, fmt.Errorf("session submitted")
	}
	return sc, nil
}

func historyToTurns(rows []tips.StoredTurn) []tips.Turn {
	out := make([]tips.Turn, 0, len(rows))
	for _, t := range rows {
		text := strings.TrimSpace(t.Text)
		if text == "" {
			continue
		}
		out = append(out, tips.Turn{Role: t.Role, Text: text})
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
