// Package replay serves step-through replays: the official solution and
// the caller's own session.
package replay

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"codritium/backend/internal/platform/httpx"

	"codritium/backend/internal/community/comments"
	"codritium/backend/internal/submissions"

	"codritium/backend/internal/sessions"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"codritium/backend/internal/auth"
)

// ReplyDeps wires the reply endpoints. They read solutions_replies for the
// official walkthroughs and session_events for the candidate's own run;
// there is no write path here — generators go through scripts, not HTTP.
// Routes registers the replay API.
func (h Handler) Routes(rt *httpx.Router) {
	rt.Handle("GET /api/challenges/{slug}/official-reply", h.getOfficialReply)
	rt.Handle("GET /api/me/replays/{slug}", h.getMyReplay)
}

type Handler struct {
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
func (h Handler) getOfficialReply(w http.ResponseWriter, r *http.Request) {
	slug := strings.TrimSpace(r.PathValue("slug"))
	if slug == "" {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "Slug required.")
		return
	}
	u := auth.FromContext(r.Context())
	if u == nil {
		httpx.Unauthorized(w)
		return
	}
	if !comments.RequireGraded(w, r, submissions.Store{Pool: h.Pool}, u, slug) {
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
	err := h.Pool.QueryRow(r.Context(), `
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
		httpx.Error(w, http.StatusNotFound, "not_found", "No official reply for this challenge yet.")
		return
	}
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}

	var rawEnvelopes []json.RawMessage
	if err := json.Unmarshal(envelopes, &rawEnvelopes); err != nil {
		httpx.Internal(w, r, err)
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
	httpx.JSON(w, http.StatusOK, resp)
}

// GetMyReplay reuses the same envelope shape to play back the candidate's
// own most recent run on a challenge. Source is session_events; the row
// for solutions_replies is never consulted here, keeping the two surfaces
// independent as the 2026-05-20 product decision required.
func (h Handler) getMyReplay(w http.ResponseWriter, r *http.Request) {
	u := auth.FromContext(r.Context())
	if u == nil {
		httpx.Unauthorized(w)
		return
	}
	slug := strings.TrimSpace(r.PathValue("slug"))
	if slug == "" {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "Slug required.")
		return
	}

	var (
		sessionID uuid.UUID
		startedAt time.Time
	)
	sessionID, startedAt, err := sessions.Store{Pool: h.Pool}.Latest(r.Context(), u.ID, slug)
	if errors.Is(err, sessions.ErrNotFound) {
		httpx.Error(w, http.StatusNotFound, "not_found", "No session for this challenge yet.")
		return
	}
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}

	rows, err := h.Pool.Query(r.Context(), `
		SELECT seq, kind, emitted_at, payload::text
		FROM session_events
		WHERE session_id = $1
		ORDER BY seq ASC`,
		sessionID,
	)
	if err != nil {
		httpx.Internal(w, r, err)
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
			httpx.Internal(w, r, err)
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
			httpx.Internal(w, r, err)
			return
		}
		envelopes = append(envelopes, raw)
	}
	if err := rows.Err(); err != nil {
		httpx.Internal(w, r, err)
		return
	}

	httpx.JSON(w, http.StatusOK, replyResponse{
		Source:    "user_session",
		Challenge: slug,
		SessionID: sessionID.String(),
		StartedAt: startedAt.Format(time.RFC3339Nano),
		Envelopes: envelopes,
	})
}

// Compile-time guard that fmt stays referenced even if all error paths
// switch to errors.New. The import is kept because future payload
// validators (e.g. kind enum check) will use it.
