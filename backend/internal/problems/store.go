package problems

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store is the only place that queries the problems table.
type Store struct{ Pool *pgxpool.Pool }

// List returns catalog rows with the given status.
func (s Store) List(ctx context.Context, status string) ([]Brief, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT slug, title, category, difficulty, COALESCE(tags, '{}'), status
		FROM problems
		WHERE status = $1
		ORDER BY difficulty, slug`, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Brief{}
	for rows.Next() {
		var b Brief
		if err := rows.Scan(&b.Slug, &b.Title, &b.Category, &b.Difficulty, &b.Tags, &b.Status); err != nil {
			return nil, err
		}
		b.RequiresPro = b.Category == CategoryCompanyPremium
		out = append(out, b)
	}
	return out, rows.Err()
}

// Get loads one problem, or ErrNotFound.
func (s Store) Get(ctx context.Context, slug string) (*Problem, error) {
	p := &Problem{Slug: slug}
	var starter []byte
	err := s.Pool.QueryRow(ctx, `
		SELECT id::text, title, category, difficulty, readme_md, strip_variant, status,
		       COALESCE(tags, '{}'), starter_files::text,
		       sample_test_filename, sample_test_content,
		       hidden_test_file, COALESCE(hidden_test_content, '')
		FROM problems WHERE slug = $1`, slug,
	).Scan(&p.ID, &p.Title, &p.Category, &p.Difficulty, &p.ReadmeMD, &p.StripVariant, &p.Status,
		&p.Tags, &starter,
		&p.SampleTestFilename, &p.SampleTestContent,
		&p.HiddenTestFile, &p.HiddenTestContent)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if p.Starter, err = ParseStarterFiles(starter); err != nil {
		return nil, err
	}
	return p, nil
}

// Exists reports whether a problem with slug exists (any status).
func (s Store) Exists(ctx context.Context, slug string) (bool, error) {
	var ok bool
	err := s.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM problems WHERE slug = $1)`, slug).Scan(&ok)
	return ok, err
}

// CatalogTotals counts published problems per difficulty and per category.
func (s Store) CatalogTotals(ctx context.Context) (byDifficulty, byCategory map[string]int, err error) {
	rows, err := s.Pool.Query(ctx, `SELECT difficulty, category FROM problems WHERE status = 'published'`)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	byDifficulty, byCategory = map[string]int{}, map[string]int{}
	for rows.Next() {
		var d, c string
		if err := rows.Scan(&d, &c); err != nil {
			return nil, nil, err
		}
		byDifficulty[d]++
		byCategory[c]++
	}
	return byDifficulty, byCategory, rows.Err()
}
