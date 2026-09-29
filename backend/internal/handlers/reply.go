package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"codritium/backend/internal/auth"
)

// ReplyDeps wires the read-only replay endpoints. Official walkthroughs
// come from solutions_replies. Candidate replays combine session_events
// with the complete transcript stored in session_messages.
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
		if u.Role != "admin" {
			var graded bool
			if err := deps.Pool.QueryRow(r.Context(), `
				SELECT EXISTS(
				  SELECT 1 FROM candidate_sessions
				  WHERE candidate_id = $1 AND challenge_id = $2 AND graded_at IS NOT NULL
				)`, u.Handle, slug).Scan(&graded); err != nil {
				http.Error(w, "gate check: "+err.Error(), http.StatusInternalServerError)
				return
			}
			if !graded {
				writeJSON(w, http.StatusForbidden, map[string]any{
					"error":          "must_complete_problem",
					"challenge_slug": slug,
				})
				return
			}
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

// GetSessionReplay returns one replay by session ID.
// The session must belong to the current user.
func GetSessionReplay(deps ReplyDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.FromContext(r.Context())
		if u == nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		sessionID, err := uuid.Parse(strings.TrimSpace(r.PathValue("id")))
		if err != nil {
			http.Error(w, "invalid session id", http.StatusBadRequest)
			return
		}

		var (
			slug      string
			startedAt time.Time
		)
		err = deps.Pool.QueryRow(r.Context(), `
			SELECT challenge_id, started_at
			FROM candidate_sessions
			WHERE session_id = $1 AND candidate_id = $2`,
			sessionID,
			u.Handle,
		).Scan(&slug, &startedAt)

		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "session not found", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(
				w,
				"lookup session: "+err.Error(),
				http.StatusInternalServerError,
			)
			return
		}

		writeSessionReplay(
			w,
			r,
			deps,
			sessionID,
			slug,
			startedAt,
		)
	}
}

// GetMyReplay returns the candidate's most recent run on a challenge.
// Structured events and complete transcript messages share one sequence
// counter, allowing the replay to return them in their original order.
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
		err := deps.Pool.QueryRow(r.Context(), `
			SELECT session_id, started_at
			FROM candidate_sessions
			WHERE candidate_id = $1 AND challenge_id = $2
			ORDER BY started_at DESC
			LIMIT 1`,
			u.Handle, slug,
		).Scan(&sessionID, &startedAt)
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "no session for this challenge yet", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, "lookup session: "+err.Error(), http.StatusInternalServerError)
			return
		}

		writeSessionReplay(
			w,
			r,
			deps,
			sessionID,
			slug,
			startedAt,
		)
	}
}

func writeSessionReplay(
	w http.ResponseWriter,
	r *http.Request,
	deps ReplyDeps,
	sessionID uuid.UUID,
	slug string,
	startedAt time.Time,
) {
	rows, err := deps.Pool.Query(r.Context(), `
		SELECT seq, kind, emitted_at, payload::text
		FROM (
			SELECT
				seq,
				kind,
				emitted_at,
				payload
			FROM session_events
			WHERE session_id = $1

			UNION ALL

			SELECT
				seq,
				'chat_message' AS kind,
				created_at AS emitted_at,
				jsonb_build_object(
					'role', role,
					'content', content
				) AS payload
			FROM session_messages
			WHERE session_id = $1
		) AS replay_items
		ORDER BY seq ASC`,
		sessionID,
	)
	if err != nil {
		http.Error(
			w,
			"load events: "+err.Error(),
			http.StatusInternalServerError,
		)
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

		if err := rows.Scan(
			&seq,
			&kind,
			&emittedAt,
			&payload,
		); err != nil {
			http.Error(
				w,
				"scan event: "+err.Error(),
				http.StatusInternalServerError,
			)
			return
		}

		envelope := userEnvelope{
			SessionID: sessionID.String(),
			Seq:       seq,
			Kind:      kind,
			EmittedAt: emittedAt.Format(time.RFC3339Nano),
			Payload:   json.RawMessage(payload),
		}

		raw, err := json.Marshal(envelope)
		if err != nil {
			http.Error(
				w,
				"encode event: "+err.Error(),
				http.StatusInternalServerError,
			)
			return
		}

		envelopes = append(envelopes, raw)
	}

	if err := rows.Err(); err != nil {
		http.Error(
			w,
			"iterate events: "+err.Error(),
			http.StatusInternalServerError,
		)
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
