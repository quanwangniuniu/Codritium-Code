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
)

type SessionsDeps struct {
	Pool *pgxpool.Pool
}

type createSessionRequest struct {
	ChallengeSlug string `json:"challenge_slug"`
	Difficulty    string `json:"difficulty,omitempty"`
	ForceNew      bool   `json:"force_new"`
}

type sessionResponse struct {
	SessionID   string `json:"session_id"`
	Created     bool   `json:"created"`
	StartedAt   string `json:"started_at"`
	ChallengeID string `json:"challenge_id"`
}

// PostSession resolves the candidate-side session_id for one (candidate,
// challenge) pair. Per decision_log Q7=C:
//   - if force_new=false and the caller has an unsubmitted session for the
//     same challenge inside the last 24 hours, that session is reused.
//   - otherwise a new candidate_sessions row is created.
//
// Returns 200 in both cases; `created` distinguishes new from reused so
// the frontend can render different toasts.
func PostSession(deps SessionsDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.FromContext(r.Context())
		if u == nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		var req createSessionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad json: "+err.Error(), http.StatusBadRequest)
			return
		}
		if req.ChallengeSlug == "" {
			http.Error(w, "challenge_slug required", http.StatusBadRequest)
			return
		}
		difficulty, err := lookupDifficulty(r.Context(), deps.Pool, req.ChallengeSlug)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				http.Error(w, "challenge not found", http.StatusNotFound)
				return
			}
			http.Error(w, "lookup: "+err.Error(), http.StatusInternalServerError)
			return
		}

		if !req.ForceNew {
			id, startedAt, ok, err := findReusableSession(r.Context(), deps.Pool, u.Handle, req.ChallengeSlug)
			if err != nil {
				http.Error(w, "reuse lookup: "+err.Error(), http.StatusInternalServerError)
				return
			}
			if ok {
				writeJSON(w, http.StatusOK, sessionResponse{
					SessionID:   id.String(),
					Created:     false,
					StartedAt:   startedAt,
					ChallengeID: req.ChallengeSlug,
				})
				return
			}
		}

		id, startedAt, err := createSession(r.Context(), deps.Pool, u.Handle, req.ChallengeSlug, difficulty)
		if err != nil {
			http.Error(w, "create: "+err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, sessionResponse{
			SessionID:   id.String(),
			Created:     true,
			StartedAt:   startedAt,
			ChallengeID: req.ChallengeSlug,
		})
	}
}

func lookupDifficulty(ctx context.Context, pool *pgxpool.Pool, slug string) (string, error) {
	var d string
	err := pool.QueryRow(ctx,
		`SELECT difficulty FROM problems WHERE slug = $1`, slug,
	).Scan(&d)
	return d, err
}

func findReusableSession(ctx context.Context, pool *pgxpool.Pool, handle, slug string) (uuid.UUID, string, bool, error) {
	var id uuid.UUID
	var startedAt string
	err := pool.QueryRow(ctx, `
		SELECT session_id, started_at::text
		FROM candidate_sessions
		WHERE candidate_id = $1
		  AND challenge_id = $2
		  AND submitted_at IS NULL
		  AND started_at > now() - interval '24 hours'
		ORDER BY started_at DESC
		LIMIT 1`,
		handle, slug,
	).Scan(&id, &startedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.UUID{}, "", false, nil
	}
	if err != nil {
		return uuid.UUID{}, "", false, err
	}
	return id, startedAt, true, nil
}

func createSession(ctx context.Context, pool *pgxpool.Pool, handle, slug, difficulty string) (uuid.UUID, string, error) {
	var id uuid.UUID
	var startedAt string
	err := pool.QueryRow(ctx, `
		INSERT INTO candidate_sessions (candidate_id, challenge_id, difficulty)
		VALUES ($1, $2, $3)
		RETURNING session_id, started_at::text`,
		handle, slug, difficulty,
	).Scan(&id, &startedAt)
	if err != nil {
		return uuid.UUID{}, "", err
	}
	return id, startedAt, nil
}

type attemptedProblem struct {
	Slug          string `json:"challenge_slug"`
	LastStartedAt string `json:"last_started_at"`
}

// ListMyAttempted returns the distinct set of challenge slugs the caller
// has ever opened a candidate_sessions row for, sorted by the most recent
// started_at descending. Used by the /problems Done filter so the row
// ordering matches "most recently worked on first".
func ListMyAttempted(deps SessionsDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.FromContext(r.Context())
		if u == nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		rows, err := deps.Pool.Query(r.Context(), `
			SELECT challenge_id, MAX(started_at)::text AS last_started_at
			FROM candidate_sessions
			WHERE candidate_id = $1
			GROUP BY challenge_id
			ORDER BY MAX(started_at) DESC`,
			u.Handle,
		)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		defer rows.Close()
		out := []attemptedProblem{}
		for rows.Next() {
			var a attemptedProblem
			if err := rows.Scan(&a.Slug, &a.LastStartedAt); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
				return
			}
			out = append(out, a)
		}
		writeJSON(w, http.StatusOK, out)
	}
}
