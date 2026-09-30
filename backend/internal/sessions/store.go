// Package sessions owns candidate_sessions: one row per attempt at a
// problem in the workspace. Every ownership check goes through Store so
// the rule lives in one place.
package sessions

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"codritium/backend/internal/auth"
)

// ErrNotFound covers both "no such session" and "not yours": callers must
// never reveal which.
var ErrNotFound = errors.New("session not found")

// ReuseWindow is how long an unsubmitted session is resumed instead of
// starting a new one.
const ReuseWindow = 24 * time.Hour

// Session is one candidate_sessions row.
type Session struct {
	ID            uuid.UUID
	UserID        uuid.UUID
	ChallengeSlug string
	Difficulty    string
	Mode          string
	StartedAt     time.Time
	SubmittedAt   *time.Time
	TotalTurns    int
}

// Attempted is a problem the user has opened, with the latest start.
type Attempted struct {
	Slug          string `json:"challenge_slug"`
	LastStartedAt string `json:"last_started_at"`
}

// Store is the only code that reads or writes candidate_sessions.
type Store struct{ Pool *pgxpool.Pool }

// GetOwned loads a session owned by u, or ErrNotFound.
func (s Store) GetOwned(ctx context.Context, id uuid.UUID, u *auth.User) (*Session, error) {
	sess := &Session{ID: id}
	err := s.Pool.QueryRow(ctx, `
		SELECT user_id, challenge_id, difficulty, COALESCE(mode, ''), started_at, submitted_at, total_turns
		FROM candidate_sessions WHERE session_id = $1`, id,
	).Scan(&sess.UserID, &sess.ChallengeSlug, &sess.Difficulty, &sess.Mode, &sess.StartedAt, &sess.SubmittedAt, &sess.TotalTurns)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if u == nil || sess.UserID != u.ID {
		return nil, ErrNotFound
	}
	return sess, nil
}

// SessionOwnedBy reports whether u owns the session.
func (s Store) SessionOwnedBy(ctx context.Context, id uuid.UUID, u *auth.User) (bool, error) {
	_, err := s.GetOwned(ctx, id, u)
	if errors.Is(err, ErrNotFound) {
		return false, nil
	}
	return err == nil, err
}

// FindReusable returns the user's latest unsubmitted session for the
// problem started within ReuseWindow.
func (s Store) FindReusable(ctx context.Context, userID uuid.UUID, slug string) (id uuid.UUID, startedAt string, ok bool, err error) {
	err = s.Pool.QueryRow(ctx, `
		SELECT session_id, started_at::text
		FROM candidate_sessions
		WHERE user_id = $1 AND challenge_id = $2 AND submitted_at IS NULL
		  AND started_at > now() - $3::interval
		ORDER BY started_at DESC
		LIMIT 1`, userID, slug, ReuseWindow.String(),
	).Scan(&id, &startedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, "", false, nil
	}
	return id, startedAt, err == nil, err
}

// Create starts a session.
func (s Store) Create(ctx context.Context, userID uuid.UUID, slug, difficulty string) (id uuid.UUID, startedAt string, err error) {
	err = s.Pool.QueryRow(ctx, `
		INSERT INTO candidate_sessions (user_id, challenge_id, difficulty)
		VALUES ($1, $2, $3)
		RETURNING session_id, started_at::text`, userID, slug, difficulty,
	).Scan(&id, &startedAt)
	return id, startedAt, err
}

// Latest returns the user's most recent session for a problem.
func (s Store) Latest(ctx context.Context, userID uuid.UUID, slug string) (id uuid.UUID, startedAt time.Time, err error) {
	err = s.Pool.QueryRow(ctx, `
		SELECT session_id, started_at FROM candidate_sessions
		WHERE user_id = $1 AND challenge_id = $2
		ORDER BY started_at DESC LIMIT 1`, userID, slug,
	).Scan(&id, &startedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, time.Time{}, ErrNotFound
	}
	return id, startedAt, err
}

// Attempted lists distinct problems the user opened, most recent first.
func (s Store) Attempted(ctx context.Context, userID uuid.UUID) ([]Attempted, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT challenge_id, MAX(started_at)::text
		FROM candidate_sessions WHERE user_id = $1
		GROUP BY challenge_id
		ORDER BY MAX(started_at) DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Attempted{}
	for rows.Next() {
		var a Attempted
		if err := rows.Scan(&a.Slug, &a.LastStartedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// MarkSubmitted closes the session.
func (s Store) MarkSubmitted(ctx context.Context, id uuid.UUID) error {
	_, err := s.Pool.Exec(ctx, `
		UPDATE candidate_sessions SET submitted_at = now()
		WHERE session_id = $1 AND submitted_at IS NULL`, id)
	return err
}

// NextTurn atomically increments total_turns and returns the new value.
// (The chat handler used to read, add one, and write back.)
func (s Store) NextTurn(ctx context.Context, id uuid.UUID) (int, error) {
	var n int
	err := s.Pool.QueryRow(ctx, `
		UPDATE candidate_sessions SET total_turns = total_turns + 1
		WHERE session_id = $1 RETURNING total_turns`, id,
	).Scan(&n)
	return n, err
}
