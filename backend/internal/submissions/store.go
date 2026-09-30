// Package submissions owns the submissions table.
package submissions

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store queries submissions.
type Store struct{ Pool *pgxpool.Pool }

// HasGraded reports whether the user has a graded submission for the
// problem. This gates per-problem discussion, replays, and note sharing.
// (It used to read candidate_sessions.graded_at, which nothing sets.)
func (s Store) HasGraded(ctx context.Context, userID uuid.UUID, slug string) (bool, error) {
	var ok bool
	err := s.Pool.QueryRow(ctx, `
		SELECT EXISTS(
		  SELECT 1 FROM submissions s JOIN problems p ON p.id = s.problem_id
		  WHERE s.user_id = $1 AND p.slug = $2 AND s.status = 'graded'
		)`, userID, slug,
	).Scan(&ok)
	return ok, err
}
