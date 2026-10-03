// Package agent is the HTTP surface of the in-editor AI agent: chat turns,
// approve/reject decisions on proposed tool calls, frontend-emitted
// events, and the per-session SSE stream. The agent runtime lives in
// package llm.
package agent

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"codritium/backend/internal/events"
	"codritium/backend/internal/llm"
	"codritium/backend/internal/platform/httpx"
)

// Handler serves the agent API.
type Handler struct {
	Pool      *pgxpool.Pool
	Registry  *llm.AgentRegistry // nil when the chat engine is disabled
	Jailbreak *llm.ChatJailbreakClassifier
	Waiter    *llm.DecisionWaiter
	Sessions  SessionOwner
	Events    *events.Store
	Text      *llm.TextBroadcaster
}

func (h Handler) Routes(rt *httpx.Router) {
	rt.Handle("POST /api/agent/chat", h.chat)
	rt.Handle("POST /api/decision", h.postDecision)
	rt.Handle("POST /api/events", h.postEvent)
	rt.Handle("GET /api/sessions/{id}/stream", h.sessionStream)
}

// chat answers 503 when no chat engine is configured.
func (h Handler) chat(w http.ResponseWriter, r *http.Request) {
	if h.Registry == nil {
		httpx.Error(w, http.StatusServiceUnavailable, "chat_disabled", "The AI chat is turned off on this server.")
		return
	}
	h.postAgentChat(w, r)
}
