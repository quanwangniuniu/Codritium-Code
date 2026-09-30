// Package seed upserts the problem catalog from seed/problems/*.json.
// It is separate from package problems so test helpers can seed a
// database without importing the module they test.
package seed

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

type problemRecord struct {
	Slug               string                       `json:"slug"`
	Title              string                       `json:"title"`
	Category           string                       `json:"category"`
	Difficulty         string                       `json:"difficulty"`
	ReadmeMD           string                       `json:"readme_md"`
	StarterFiles       map[string]map[string]string `json:"starter_files"`
	HiddenTestFilename string                       `json:"hidden_test_filename"`
	HiddenTestContent  string                       `json:"hidden_test_content"`
	StripVariant       string                       `json:"strip_variant"`
	SoulPrebake        string                       `json:"soul_prebake"`
	Tags               []string                     `json:"tags,omitempty"`
	SampleTestFilename string                       `json:"sample_test_filename,omitempty"`
	SampleTestContent  string                       `json:"sample_test_content,omitempty"`
}

// FromDir loads every *.json file in dir and upserts a problem row.
// hidden_test_content is stored alongside the problem and MUST be filtered
// out of the candidate-facing API (see handlers/problems.go). status is
// intentionally not part of the upsert payload — it is operator-managed via
// scripts/admin_sql.md and would otherwise reset on every boot.
func FromDir(ctx context.Context, pool *pgxpool.Pool, dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read seed dir: %w", err)
	}

	seeded := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var rec problemRecord
		if err := json.Unmarshal(body, &rec); err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
		if rec.StripVariant == "" {
			rec.StripVariant = "both"
		}
		if rec.HiddenTestFilename == "" {
			rec.HiddenTestFilename = "test_answer.py"
		}
		if rec.Tags == nil {
			rec.Tags = []string{}
		}

		if _, leaked := rec.StarterFiles[rec.HiddenTestFilename]; leaked {
			log.Printf("seed: %s — hidden_test_filename %q present in starter_files, stripping to prevent leak",
				rec.Slug, rec.HiddenTestFilename)
			delete(rec.StarterFiles, rec.HiddenTestFilename)
		}

		starterJSON, err := json.Marshal(rec.StarterFiles)
		if err != nil {
			return err
		}

		_, err = pool.Exec(ctx, `
			INSERT INTO problems (
				slug, title, category, difficulty, readme_md, starter_files, strip_variant,
				hidden_test_file, hidden_test_content, soul_prebake,
				tags, sample_test_filename, sample_test_content
			)
			VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7, $8, $9, $10, $11, $12, $13)
			ON CONFLICT (slug) DO UPDATE SET
				title = EXCLUDED.title,
				category = EXCLUDED.category,
				difficulty = EXCLUDED.difficulty,
				readme_md = EXCLUDED.readme_md,
				starter_files = EXCLUDED.starter_files,
				strip_variant = EXCLUDED.strip_variant,
				hidden_test_file = EXCLUDED.hidden_test_file,
				hidden_test_content = EXCLUDED.hidden_test_content,
				soul_prebake = EXCLUDED.soul_prebake,
				tags = EXCLUDED.tags,
				sample_test_filename = EXCLUDED.sample_test_filename,
				sample_test_content = EXCLUDED.sample_test_content
		`, rec.Slug, rec.Title, rec.Category, rec.Difficulty, rec.ReadmeMD,
			string(starterJSON), rec.StripVariant, rec.HiddenTestFilename, rec.HiddenTestContent, rec.SoulPrebake,
			rec.Tags, nullableString(rec.SampleTestFilename), nullableString(rec.SampleTestContent))
		if err != nil {
			return fmt.Errorf("upsert %s: %w", rec.Slug, err)
		}
		seeded++
	}
	log.Printf("seed: upserted %d problems from %s", seeded, dir)
	return nil
}

// nullableString returns nil for empty input so the row stores NULL rather
// than an empty string. Keeps "no sample test" distinct from "sample test that
// is the empty string" downstream.
func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}
