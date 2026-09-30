package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"codritium/backend/internal/sessions"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"codritium/backend/internal/auth"
)

type NotesDeps struct {
	Pool *pgxpool.Pool
}

type noteItem struct {
	ID                uuid.UUID  `json:"id"`
	SessionID         uuid.UUID  `json:"session_id"`
	Body              string     `json:"body"`
	SharedToCommentID *uuid.UUID `json:"shared_to_comment_id,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

// ListNotes returns every note the caller has attached to the given session.
// Sessions are owned by their candidate_id (handle) — a caller can never see
// another candidate's notes.
func ListNotes(deps NotesDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.FromContext(r.Context())
		if u == nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		sessionID, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			http.Error(w, "bad session id", http.StatusBadRequest)
			return
		}
		if err := requireSessionOwner(r, deps.Pool, u, sessionID); err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}

		rows, err := deps.Pool.Query(r.Context(), `
			SELECT id, session_id, body, shared_to_comment_id, created_at, updated_at
			FROM session_notes
			WHERE session_id = $1
			ORDER BY created_at ASC`, sessionID)
		if err != nil {
			http.Error(w, "list notes: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer rows.Close()
		out := make([]noteItem, 0)
		for rows.Next() {
			var it noteItem
			if err := rows.Scan(&it.ID, &it.SessionID, &it.Body, &it.SharedToCommentID,
				&it.CreatedAt, &it.UpdatedAt); err != nil {
				http.Error(w, "scan: "+err.Error(), http.StatusInternalServerError)
				return
			}
			out = append(out, it)
		}
		writeJSON(w, http.StatusOK, out)
	}
}

type noteBody struct {
	Body string `json:"body"`
}

// CreateNote inserts a new note on the caller's own session.
func CreateNote(deps NotesDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.FromContext(r.Context())
		if u == nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		sessionID, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			http.Error(w, "bad session id", http.StatusBadRequest)
			return
		}
		if err := requireSessionOwner(r, deps.Pool, u, sessionID); err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		var req noteBody
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad json: "+err.Error(), http.StatusBadRequest)
			return
		}
		body := strings.TrimSpace(req.Body)
		if body == "" {
			http.Error(w, "body required", http.StatusBadRequest)
			return
		}
		var id uuid.UUID
		var createdAt time.Time
		err = deps.Pool.QueryRow(r.Context(), `
			INSERT INTO session_notes (session_id, user_id, body)
			VALUES ($1, $2, $3)
			RETURNING id, created_at`,
			sessionID, u.ID, body,
		).Scan(&id, &createdAt)
		if err != nil {
			http.Error(w, "create note: "+err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{
			"id":         id,
			"session_id": sessionID,
			"created_at": createdAt,
		})
	}
}

// UpdateNote rewrites the body of a note the caller owns and bumps the
// updated_at column.
func UpdateNote(deps NotesDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.FromContext(r.Context())
		if u == nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		noteID, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			http.Error(w, "bad id", http.StatusBadRequest)
			return
		}
		var req noteBody
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad json: "+err.Error(), http.StatusBadRequest)
			return
		}
		body := strings.TrimSpace(req.Body)
		if body == "" {
			http.Error(w, "body required", http.StatusBadRequest)
			return
		}
		tag, err := deps.Pool.Exec(r.Context(), `
			UPDATE session_notes SET body = $3, updated_at = now()
			WHERE id = $1 AND user_id = $2`,
			noteID, u.ID, body)
		if err != nil {
			http.Error(w, "update: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if tag.RowsAffected() == 0 {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// DeleteNote removes a note the caller owns. Hard delete — notes are
// candidate-private; there is no audit requirement to keep the body around.
func DeleteNote(deps NotesDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.FromContext(r.Context())
		if u == nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		noteID, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			http.Error(w, "bad id", http.StatusBadRequest)
			return
		}
		tag, err := deps.Pool.Exec(r.Context(),
			`DELETE FROM session_notes WHERE id = $1 AND user_id = $2`,
			noteID, u.ID)
		if err != nil {
			http.Error(w, "delete: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if tag.RowsAffected() == 0 {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// ShareNote publishes the note body as a comment under the problem the
// session belongs to, and back-links the note via shared_to_comment_id so
// the candidate can see the share status. Honors the graded gate; a note
// can't be shared until the candidate has finished the problem at least
// once.
func ShareNote(deps NotesDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.FromContext(r.Context())
		if u == nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		noteID, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			http.Error(w, "bad id", http.StatusBadRequest)
			return
		}

		var (
			sessionID uuid.UUID
			noteBody  string
			challenge string
			already   *uuid.UUID
		)
		err = deps.Pool.QueryRow(r.Context(), `
			SELECT n.session_id, n.body, cs.challenge_id, n.shared_to_comment_id
			FROM session_notes n
			JOIN candidate_sessions cs ON cs.session_id = n.session_id
			WHERE n.id = $1 AND n.user_id = $2`,
			noteID, u.ID,
		).Scan(&sessionID, &noteBody, &challenge, &already)
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, "lookup note: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if already != nil {
			writeJSON(w, http.StatusConflict, map[string]any{
				"error":            "already_shared",
				"existing_comment": *already,
			})
			return
		}
		if err := requireGradedOrAdmin(r, deps.Pool, u, challenge); errors.Is(err, errNotGraded) {
			writeJSON(w, http.StatusForbidden, map[string]any{
				"error":          "must_complete_problem",
				"challenge_slug": challenge,
			})
			return
		} else if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		tx, err := deps.Pool.BeginTx(r.Context(), pgx.TxOptions{})
		if err != nil {
			http.Error(w, "tx begin: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer tx.Rollback(r.Context())

		var commentID uuid.UUID
		err = tx.QueryRow(r.Context(), `
			INSERT INTO comments (problem_slug, user_id, body)
			VALUES ($1, $2, $3)
			RETURNING id`,
			challenge, u.ID, noteBody,
		).Scan(&commentID)
		if err != nil {
			http.Error(w, "create comment: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if _, err := tx.Exec(r.Context(),
			`UPDATE session_notes SET shared_to_comment_id = $2 WHERE id = $1`,
			noteID, commentID,
		); err != nil {
			http.Error(w, "link note: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if err := tx.Commit(r.Context()); err != nil {
			http.Error(w, "tx commit: "+err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{
			"comment_id":     commentID,
			"challenge_slug": challenge,
		})
	}
}

func requireSessionOwner(r *http.Request, pool *pgxpool.Pool, u *auth.User, sessionID uuid.UUID) error {
	_, err := sessions.Store{Pool: pool}.GetOwned(r.Context(), sessionID, u)
	return err
}
