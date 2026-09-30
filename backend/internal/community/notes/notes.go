// Package notes is a candidate's private per-session notes, which can be
// shared as a problem comment once the problem is graded.
package notes

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"codritium/backend/internal/community/comments"
	"codritium/backend/internal/platform/httpx"
	"codritium/backend/internal/submissions"

	"codritium/backend/internal/sessions"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"codritium/backend/internal/auth"
)

// Routes registers the notes API.
func (h Handler) Routes(rt *httpx.Router) {
	rt.Handle("GET /api/sessions/{id}/notes", h.listNotes)
	rt.Handle("POST /api/sessions/{id}/notes", h.createNote)
	rt.Handle("PUT /api/notes/{id}", h.updateNote)
	rt.Handle("DELETE /api/notes/{id}", h.deleteNote)
	rt.Handle("POST /api/notes/{id}/share", h.shareNote)
}

type Handler struct {
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
func (h Handler) listNotes(w http.ResponseWriter, r *http.Request) {
	u := auth.FromContext(r.Context())
	if u == nil {
		httpx.Unauthorized(w)
		return
	}
	sessionID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "Bad session id.")
		return
	}
	if err := requireSessionOwner(r, h.Pool, u, sessionID); err != nil {
		httpx.Error(w, http.StatusNotFound, "not_found", "Not found.")
		return
	}

	rows, err := h.Pool.Query(r.Context(), `
		SELECT id, session_id, body, shared_to_comment_id, created_at, updated_at
		FROM session_notes
		WHERE session_id = $1
		ORDER BY created_at ASC`, sessionID)
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	defer rows.Close()
	out := make([]noteItem, 0)
	for rows.Next() {
		var it noteItem
		if err := rows.Scan(&it.ID, &it.SessionID, &it.Body, &it.SharedToCommentID,
			&it.CreatedAt, &it.UpdatedAt); err != nil {
			httpx.Internal(w, r, err)
			return
		}
		out = append(out, it)
	}
	httpx.JSON(w, http.StatusOK, out)
}

type noteBody struct {
	Body string `json:"body"`
}

// CreateNote inserts a new note on the caller's own session.
func (h Handler) createNote(w http.ResponseWriter, r *http.Request) {
	u := auth.FromContext(r.Context())
	if u == nil {
		httpx.Unauthorized(w)
		return
	}
	sessionID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "Bad session id.")
		return
	}
	if err := requireSessionOwner(r, h.Pool, u, sessionID); err != nil {
		httpx.Error(w, http.StatusNotFound, "not_found", "Not found.")
		return
	}
	var req noteBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "Bad json.")
		return
	}
	body := strings.TrimSpace(req.Body)
	if body == "" {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "Body required.")
		return
	}
	var id uuid.UUID
	var createdAt time.Time
	err = h.Pool.QueryRow(r.Context(), `
		INSERT INTO session_notes (session_id, user_id, body)
		VALUES ($1, $2, $3)
		RETURNING id, created_at`,
		sessionID, u.ID, body,
	).Scan(&id, &createdAt)
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]any{
		"id":         id,
		"session_id": sessionID,
		"created_at": createdAt,
	})
}

// UpdateNote rewrites the body of a note the caller owns and bumps the
// updated_at column.
func (h Handler) updateNote(w http.ResponseWriter, r *http.Request) {
	u := auth.FromContext(r.Context())
	if u == nil {
		httpx.Unauthorized(w)
		return
	}
	noteID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "Bad id.")
		return
	}
	var req noteBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "Bad json.")
		return
	}
	body := strings.TrimSpace(req.Body)
	if body == "" {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "Body required.")
		return
	}
	tag, err := h.Pool.Exec(r.Context(), `
		UPDATE session_notes SET body = $3, updated_at = now()
		WHERE id = $1 AND user_id = $2`,
		noteID, u.ID, body)
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	if tag.RowsAffected() == 0 {
		httpx.Error(w, http.StatusNotFound, "not_found", "Not found.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// DeleteNote removes a note the caller owns. Hard delete — notes are
// candidate-private; there is no audit requirement to keep the body around.
func (h Handler) deleteNote(w http.ResponseWriter, r *http.Request) {
	u := auth.FromContext(r.Context())
	if u == nil {
		httpx.Unauthorized(w)
		return
	}
	noteID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "Bad id.")
		return
	}
	tag, err := h.Pool.Exec(r.Context(),
		`DELETE FROM session_notes WHERE id = $1 AND user_id = $2`,
		noteID, u.ID)
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	if tag.RowsAffected() == 0 {
		httpx.Error(w, http.StatusNotFound, "not_found", "Not found.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ShareNote publishes the note body as a comment under the problem the
// session belongs to, and back-links the note via shared_to_comment_id so
// the candidate can see the share status. Honors the graded gate; a note
// can't be shared until the candidate has finished the problem at least
// once.
func (h Handler) shareNote(w http.ResponseWriter, r *http.Request) {
	u := auth.FromContext(r.Context())
	if u == nil {
		httpx.Unauthorized(w)
		return
	}
	noteID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "Bad id.")
		return
	}

	var (
		sessionID uuid.UUID
		noteBody  string
		challenge string
		already   *uuid.UUID
	)
	err = h.Pool.QueryRow(r.Context(), `
		SELECT n.session_id, n.body, cs.challenge_id, n.shared_to_comment_id
		FROM session_notes n
		JOIN candidate_sessions cs ON cs.session_id = n.session_id
		WHERE n.id = $1 AND n.user_id = $2`,
		noteID, u.ID,
	).Scan(&sessionID, &noteBody, &challenge, &already)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, http.StatusNotFound, "not_found", "Not found.")
		return
	}
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	if already != nil {
		httpx.JSON(w, http.StatusConflict, map[string]any{
			"error":            "already_shared",
			"existing_comment": *already,
		})
		return
	}
	if !comments.RequireGraded(w, r, submissions.Store{Pool: h.Pool}, u, challenge) {
		return
	}

	tx, err := h.Pool.BeginTx(r.Context(), pgx.TxOptions{})
	if err != nil {
		httpx.Internal(w, r, err)
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
		httpx.Internal(w, r, err)
		return
	}
	if _, err := tx.Exec(r.Context(),
		`UPDATE session_notes SET shared_to_comment_id = $2 WHERE id = $1`,
		noteID, commentID,
	); err != nil {
		httpx.Internal(w, r, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		httpx.Internal(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]any{
		"comment_id":     commentID,
		"challenge_slug": challenge,
	})
}

func requireSessionOwner(r *http.Request, pool *pgxpool.Pool, u *auth.User, sessionID uuid.UUID) error {
	_, err := sessions.Store{Pool: pool}.GetOwned(r.Context(), sessionID, u)
	return err
}
