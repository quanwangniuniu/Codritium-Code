// Package submissions owns the submissions table and its HTTP API. Grading
// itself runs in package grading.
package submissions

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound covers both missing rows and other users' rows.
var ErrNotFound = errors.New("submission not found")

// Store is the only code that reads or writes submissions (apart from the
// grading worker's result update).
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

// CountSince counts the user's submissions since t (rate limiting).
func (s Store) CountSince(ctx context.Context, userID uuid.UUID, since time.Time) (int, error) {
	var n int
	err := s.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM submissions WHERE user_id = $1 AND submitted_at >= $2`,
		userID, since).Scan(&n)
	return n, err
}

// Draft is a submission about to be graded.
type Draft struct {
	UserID    uuid.UUID
	ProblemID uuid.UUID
	Variant   string
	Code      map[string]string
	SessionID *uuid.UUID
}

// Create inserts a submission in the grading state.
func (s Store) Create(ctx context.Context, d Draft) (uuid.UUID, error) {
	code, err := json.Marshal(d.Code)
	if err != nil {
		return uuid.Nil, err
	}
	var id uuid.UUID
	err = s.Pool.QueryRow(ctx, `
		INSERT INTO submissions (user_id, problem_id, variant, code_files, session_id, status, submitted_at)
		VALUES ($1, $2, $3, $4::jsonb, $5, 'grading', now()) RETURNING id`,
		d.UserID, d.ProblemID, d.Variant, string(code), d.SessionID,
	).Scan(&id)
	return id, err
}

// Resubmit moves the user's existing submission id back to grading with
// new code. ErrNotFound when id is not theirs.
func (s Store) Resubmit(ctx context.Context, id uuid.UUID, d Draft) error {
	code, err := json.Marshal(d.Code)
	if err != nil {
		return err
	}
	tag, err := s.Pool.Exec(ctx, `
		UPDATE submissions SET code_files = $2::jsonb, variant = $3,
		       session_id = COALESCE($4, session_id), status = 'grading', submitted_at = now()
		WHERE id = $1 AND user_id = $5`,
		id, string(code), d.Variant, d.SessionID, d.UserID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Summary is one row of the user's submission list.
type Summary struct {
	ID           string     `json:"id"`
	ProblemSlug  string     `json:"problem_slug"`
	ProblemTitle string     `json:"problem_title"`
	Category     string     `json:"category"`
	Difficulty   string     `json:"difficulty"`
	Status       string     `json:"status"`
	Variant      string     `json:"variant"`
	FinalScore   *float64   `json:"final_score"`
	SubmittedAt  time.Time  `json:"submitted_at"`
	GradedAt     *time.Time `json:"graded_at"`
}

// ListMine returns the user's newest submissions.
func (s Store) ListMine(ctx context.Context, userID uuid.UUID, limit int) ([]Summary, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT s.id, p.slug, p.title, p.category, p.difficulty,
		       s.status, s.variant, s.final_score, s.submitted_at, s.graded_at
		FROM submissions s JOIN problems p ON p.id = s.problem_id
		WHERE s.user_id = $1
		ORDER BY s.submitted_at DESC
		LIMIT $2`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Summary{}
	for rows.Next() {
		var it Summary
		if err := rows.Scan(&it.ID, &it.ProblemSlug, &it.ProblemTitle, &it.Category, &it.Difficulty,
			&it.Status, &it.Variant, &it.FinalScore, &it.SubmittedAt, &it.GradedAt); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

// Detail is one submission with code, tests, and scores.
type Detail struct {
	ID          uuid.UUID         `json:"id"`
	UserID      uuid.UUID         `json:"user_id"`
	ProblemSlug string            `json:"problem_slug"`
	Status      string            `json:"status"`
	Variant     string            `json:"variant"`
	Code        map[string]string `json:"code_files"`
	TestResults json.RawMessage   `json:"test_results"`
	Scores      json.RawMessage   `json:"scores"`
	FinalScore  *float64          `json:"final_score"`
	SubmittedAt *time.Time        `json:"submitted_at"`
	GradedAt    *time.Time        `json:"graded_at"`
}

// GetMine loads one of the user's submissions, or ErrNotFound.
func (s Store) GetMine(ctx context.Context, id, userID uuid.UUID) (*Detail, error) {
	d := &Detail{ID: id}
	var code, tests, scores []byte
	err := s.Pool.QueryRow(ctx, `
		SELECT s.user_id, p.slug, s.status, s.variant, s.code_files::text,
		       COALESCE(s.test_results::text, 'null'), COALESCE(s.scores::text, 'null'),
		       s.final_score, s.submitted_at, s.graded_at
		FROM submissions s JOIN problems p ON p.id = s.problem_id
		WHERE s.id = $1 AND s.user_id = $2`, id, userID,
	).Scan(&d.UserID, &d.ProblemSlug, &d.Status, &d.Variant, &code, &tests, &scores,
		&d.FinalScore, &d.SubmittedAt, &d.GradedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal(code, &d.Code)
	d.TestResults, d.Scores = tests, scores
	return d, nil
}

// DeleteMine deletes one of the user's submissions, or ErrNotFound.
func (s Store) DeleteMine(ctx context.Context, id, userID uuid.UUID) error {
	tag, err := s.Pool.Exec(ctx, `DELETE FROM submissions WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
