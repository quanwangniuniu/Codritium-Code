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

type ChatV2Deps struct {
	Pool      *pgxpool.Pool
	Registry  *llm.AgentRegistry
	Jailbreak *llm.ChatJailbreakClassifier
}

type chatV2Request struct {
	SessionID string            `json:"session_id"`
	Message   string            `json:"message"`
	// Files is the candidate's current workspace snapshot. When non-empty
	// it overrides the Agent's cached workspace before RunTurn — the
	// candidate may have made manual edits in the editor that the
	// backend hasn't seen via FileEdit tool_results.
	Files map[string]string `json:"files,omitempty"`
}

type chatV2Response struct {
	TurnIndex int    `json:"turn_index"`
	Accepted  bool   `json:"accepted"`
	SessionID string `json:"session_id"`
}

// PostChatV2 starts one RunTurn against the candidate's session and
// returns 202 immediately. The actual events + assistant text are
// delivered through the SSE stream at GET /api/sessions/{id}/stream;
// the chat goroutine uses context.Background() so the HTTP request
// can terminate without aborting the turn.
//
// Auth: caller must own the session (candidate_id == handle, per
// decision_log D3).
func PostChatV2(deps ChatV2Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.FromContext(r.Context())
		if u == nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		var req chatV2Request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad json: "+err.Error(), http.StatusBadRequest)
			return
		}
		sessionID, err := uuid.Parse(req.SessionID)
		if err != nil {
			http.Error(w, "session_id must be a uuid", http.StatusBadRequest)
			return
		}
		if req.Message == "" {
			http.Error(w, "message required", http.StatusBadRequest)
			return
		}

		// Severe-jailbreak gate: relax-threshold filter intercepts only
		// attempts to access the hidden test or forge a grader result.
		// Legitimate coding questions never match.
		if deps.Jailbreak != nil && deps.Jailbreak.IsSevere(req.Message) {
			writeJSON(w, http.StatusForbidden, map[string]any{
				"error":  "request_blocked",
				"reason": "your request was blocked for security reasons",
			})
			return
		}

		challengeSlug, turnIndex, err := lookupSessionForChat(r.Context(), deps.Pool, u.Handle, sessionID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				// Don't leak whether the session belongs to someone else.
				http.Error(w, "session not found", http.StatusNotFound)
				return
			}
			http.Error(w, "lookup: "+err.Error(), http.StatusInternalServerError)
			return
		}

		agent, err := deps.Registry.GetOrCreate(r.Context(), sessionID, challengeSlug)
		if err != nil {
			http.Error(w, "agent: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Sync the agent's workspace with the candidate's current state.
		// The frontend is the source of truth — they may have edited the
		// editor manually since the last tool_result landed.
		if len(req.Files) > 0 && agent.Workspace != nil {
			for path, content := range req.Files {
				agent.Workspace.Files[path] = content
			}
		}

		// Bump turn counter on the session row up-front so concurrent chats
		// see a monotonic turn_index even if RunTurn is still running.
		nextTurn := turnIndex + 1
		if _, err := deps.Pool.Exec(r.Context(),
			`UPDATE candidate_sessions SET total_turns = $2 WHERE session_id = $1`,
			sessionID, nextTurn,
		); err != nil {
			http.Error(w, "bump turns: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Spawn the turn in a detached context. The HTTP request can
		// return while RunTurn is still busy; events fan-out through
		// events.Store + TextBroadcaster reach any SSE listener.
		go func() {
			_, _ = agent.RunTurn(context.Background(), sessionID, nextTurn, req.Message)
		}()

		writeJSON(w, http.StatusAccepted, chatV2Response{
			TurnIndex: nextTurn,
			Accepted:  true,
			SessionID: sessionID.String(),
		})
	}
}

// lookupSessionForChat returns (challenge_slug, current_total_turns) when
// sessionID exists and belongs to the candidate identified by handle.
// pgx.ErrNoRows otherwise — handler returns 404 in both cases (no leak).
func lookupSessionForChat(ctx context.Context, pool *pgxpool.Pool, handle string, sessionID uuid.UUID) (string, int, error) {
	var challenge string
	var totalTurns int
	var candidateID string
	err := pool.QueryRow(ctx, `
		SELECT candidate_id, challenge_id, total_turns
		FROM candidate_sessions WHERE session_id = $1`,
		sessionID,
	).Scan(&candidateID, &challenge, &totalTurns)
	if err != nil {
		return "", 0, err
	}
	if candidateID != handle {
		return "", 0, pgx.ErrNoRows
	}
	return challenge, totalTurns, nil
}
