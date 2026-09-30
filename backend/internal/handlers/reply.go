package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"codritium/backend/internal/sessions"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"codritium/backend/internal/auth"
)

// ReplyDeps wires the reply endpoints. They read solutions_replies for the
// official walkthroughs and session_events for the candidate's own run;
// there is no write path here — generators go through scripts, not HTTP.
type ReplyDeps struct {
	Pool *pgxpool.Pool
}

type replyResponse struct {
	Source        string            `json:"source"`
	Challenge     string            `json:"challenge_slug"`
	Generator     string            `json:"generator_model,omitempty"`
	Candidate     *int              `json:"candidate_index,omitempty"`
	CreatedAt     string            `json:"created_at,omitempty"`
	SessionID     string            `json:"session_id,omitempty"`
	StartedAt     string            `json:"started_at,omitempty"`
	Envelopes     []json.RawMessage `json:"envelopes"`
	Files         json.RawMessage   `json:"files,omitempty"`
	ExplanationMD string            `json:"explanation_md,omitempty"`
	StarterFiles  json.RawMessage   `json:"starter_files,omitempty"`
}

type userEnvelope struct {
	SessionID string          `json:"session_id"`
	Seq       int64           `json:"seq"`
	Kind      string          `json:"kind"`
	EmittedAt string          `json:"emitted_at"`
	Payload   json.RawMessage `json:"payload"`
}

// GetOfficialReply returns the most recently created official walkthrough
// for a challenge slug. Callers must have completed (graded) the challenge
// at least once — admins skip the gate so they can audit the walkthrough.
// Retention is unbounded; when multiple candidates exist for the same
// challenge, the one with the highest created_at wins. Editors should
// delete or supersede old rows rather than ranking them.
func GetOfficialReply(deps ReplyDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slug := strings.TrimSpace(r.PathValue("slug"))
		if slug == "" {
			http.Error(w, "slug required", http.StatusBadRequest)
			return
		}
		u := auth.FromContext(r.Context())
		if u == nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if err := requireGradedOrAdmin(r, deps.Pool, u, slug); errors.Is(err, errNotGraded) {
			writeJSON(w, http.StatusForbidden, map[string]any{
				"error":          "must_complete_problem",
				"challenge_slug": slug,
			})
			return
		} else if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		var (
			source        string
			envelopes     []byte
			generator     *string
			candidate     *int
			createdAt     string
			files         []byte
			explanationMD *string
			starterFiles  []byte
		)
		err := deps.Pool.QueryRow(r.Context(), `
			SELECT source, envelopes::text, generator_model, candidate_index, created_at::text,
			       COALESCE(files::text, '{}'), explanation_md,
			       COALESCE(starter_files::text, '{}')
			FROM solutions_replies
			WHERE challenge_id = $1 AND source = 'official_ai'
			ORDER BY created_at DESC
			LIMIT 1`,
			slug,
		).Scan(&source, &envelopes, &generator, &candidate, &createdAt,
			&files, &explanationMD, &starterFiles)
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "no official reply for this challenge yet", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, "load reply: "+err.Error(), http.StatusInternalServerError)
			return
		}

		var rawEnvelopes []json.RawMessage
		if err := json.Unmarshal(envelopes, &rawEnvelopes); err != nil {
			http.Error(w, "decode envelopes: "+err.Error(), http.StatusInternalServerError)
			return
		}

		resp := replyResponse{
			Source:       source,
			Challenge:    slug,
			CreatedAt:    createdAt,
			Envelopes:    rawEnvelopes,
			Files:        json.RawMessage(files),
			StarterFiles: json.RawMessage(starterFiles),
		}
		if generator != nil {
			resp.Generator = *generator
		}
		if candidate != nil {
			resp.Candidate = candidate
		}
		if explanationMD != nil {
			resp.ExplanationMD = *explanationMD
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

// GetMyReplay reuses the same envelope shape to play back the candidate's
// own most recent run on a challenge. Source is session_events; the row
// for solutions_replies is never consulted here, keeping the two surfaces
// independent as the 2026-05-20 product decision required.
func GetMyReplay(deps ReplyDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.FromContext(r.Context())
		if u == nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		slug := strings.TrimSpace(r.PathValue("slug"))
		if slug == "" {
			http.Error(w, "slug required", http.StatusBadRequest)
			return
		}

		var (
			sessionID uuid.UUID
			startedAt time.Time
		)
		sessionID, startedAt, err := sessions.Store{Pool: deps.Pool}.Latest(r.Context(), u.ID, slug)
		if errors.Is(err, sessions.ErrNotFound) {
			http.Error(w, "no session for this challenge yet", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, "lookup session: "+err.Error(), http.StatusInternalServerError)
			return
		}

		rows, err := deps.Pool.Query(r.Context(), `
			SELECT seq, kind, emitted_at, payload::text
			FROM session_events
			WHERE session_id = $1
			ORDER BY seq ASC`,
			sessionID,
		)
		if err != nil {
			http.Error(w, "load events: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		envelopes := make([]json.RawMessage, 0)
		for rows.Next() {
			var (
				seq       int64
				kind      string
				emittedAt time.Time
				payload   []byte
			)
			if err := rows.Scan(&seq, &kind, &emittedAt, &payload); err != nil {
				http.Error(w, "scan event: "+err.Error(), http.StatusInternalServerError)
				return
			}
			env := userEnvelope{
				SessionID: sessionID.String(),
				Seq:       seq,
				Kind:      kind,
				EmittedAt: emittedAt.Format(time.RFC3339Nano),
				Payload:   json.RawMessage(payload),
			}
			raw, err := json.Marshal(env)
			if err != nil {
				http.Error(w, "encode event: "+err.Error(), http.StatusInternalServerError)
				return
			}
			envelopes = append(envelopes, raw)
		}
		if err := rows.Err(); err != nil {
			http.Error(w, "iterate events: "+err.Error(), http.StatusInternalServerError)
			return
		}

		writeJSON(w, http.StatusOK, replyResponse{
			Source:    "user_session",
			Challenge: slug,
			SessionID: sessionID.String(),
			StartedAt: startedAt.Format(time.RFC3339Nano),
			Envelopes: envelopes,
		})
	}
}

// Compile-time guard that fmt stays referenced even if all error paths
// switch to errors.New. The import is kept because future payload
// validators (e.g. kind enum check) will use it.
var _ = fmt.Sprintf
