package problems

import (
	"context"
	"errors"
	"strings"

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

// userStatusSQL classifies a problem for the user in $1 (NULL = anonymous).
// Solved means a graded submission; attempted means a session was started.
const userStatusSQL = `
	CASE
		WHEN $1::uuid IS NULL THEN 'todo'
		WHEN EXISTS (SELECT 1 FROM submissions s
		             WHERE s.problem_id = p.id AND s.user_id = $1::uuid AND s.status = 'graded') THEN 'solved'
		WHEN EXISTS (SELECT 1 FROM candidate_sessions cs
		             WHERE cs.challenge_id = p.slug AND cs.user_id = $1::uuid) THEN 'attempted'
		ELSE 'todo'
	END`

// likeEscaper makes user input literal inside an ILIKE pattern.
var likeEscaper = strings.NewReplacer(`\`, `\`, `%`, `\%`, `_`, `\_`)

// Search returns one page of the catalog, easiest first.
func (s Store) Search(ctx context.Context, sp SearchParams) (*SearchPage, error) {
	var userID *string
	if sp.UserID != "" {
		userID = &sp.UserID
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT slug, title, category, difficulty, tags, status, user_status, COUNT(*) OVER ()
		FROM (
			SELECT p.slug, p.title, p.category, p.difficulty, COALESCE(p.tags, '{}') AS tags, p.status,
			       `+userStatusSQL+` AS user_status
			FROM problems p
			WHERE p.status = $2
			  AND ($3 = '' OR p.category = $3)
			  AND ($4 = '' OR p.difficulty = $4)
			  AND ($5 = '' OR p.tags @> ARRAY[$5])
			  AND ($6 = '' OR p.title ILIKE '%' || $6 || '%')
		) q
		WHERE $7 = '' OR user_status = $7
		ORDER BY array_position(ARRAY['easy', 'medium', 'hard'], difficulty), slug
		LIMIT $8 OFFSET $9`,
		userID, sp.Status, sp.Category, sp.Difficulty, sp.Tag,
		likeEscaper.Replace(sp.Query), sp.UserStatus, sp.Limit, sp.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	page := &SearchPage{Items: []SearchItem{}}
	for rows.Next() {
		var it SearchItem
		if err := rows.Scan(&it.Slug, &it.Title, &it.Category, &it.Difficulty, &it.Tags, &it.Status,
			&it.UserStatus, &page.Total); err != nil {
			return nil, err
		}
		it.RequiresPro = it.Category == CategoryCompanyPremium
		page.Items = append(page.Items, it)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if next := sp.Offset + len(page.Items); next < page.Total {
		page.NextOffset = &next
	}
	return page, nil
}

// Facets counts published problems per tag and category, plus how many the
// user has solved. Tags are ordered most-used first.
func (s Store) Facets(ctx context.Context, userID string) (*Facets, error) {
	f := &Facets{Tags: []FacetCount{}, Categories: []FacetCount{}}
	rows, err := s.Pool.Query(ctx, `
		SELECT 'tag', t, COUNT(*) FROM problems, unnest(tags) AS t
		WHERE status = 'published' GROUP BY t
		UNION ALL
		SELECT 'category', category, COUNT(*) FROM problems
		WHERE status = 'published' GROUP BY category
		ORDER BY 1, 3 DESC, 2`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var kind string
		var c FacetCount
		if err := rows.Scan(&kind, &c.Value, &c.Count); err != nil {
			return nil, err
		}
		if kind == "tag" {
			f.Tags = append(f.Tags, c)
		} else {
			f.Categories = append(f.Categories, c)
			f.Total += c.Count
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if userID == "" {
		return f, nil
	}
	err = s.Pool.QueryRow(ctx, `
		SELECT COUNT(DISTINCT s.problem_id)
		FROM submissions s JOIN problems p ON p.id = s.problem_id
		WHERE s.user_id = $1::uuid AND s.status = 'graded' AND p.status = 'published'`, userID).Scan(&f.Solved)
	return f, err
}
