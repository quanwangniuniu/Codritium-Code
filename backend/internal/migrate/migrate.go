package migrate

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Run applies every .sql file in migrationsDir in lexicographic order,
// then every .sql file in seedDir (if non-empty), recording applied files
// in a schema_migrations table. Idempotent.
func Run(ctx context.Context, pool *pgxpool.Pool, migrationsDir, seedDir string) error {
	if _, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			filename TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ DEFAULT now()
		)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	if err := applyDir(ctx, pool, migrationsDir, "migration"); err != nil {
		return err
	}
	if seedDir != "" {
		if err := applyDir(ctx, pool, seedDir, "seed"); err != nil {
			return err
		}
	}
	return nil
}

func applyDir(ctx context.Context, pool *pgxpool.Pool, dir, kind string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read %s dir %s: %w", kind, dir, err)
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	for _, name := range files {
		key := kind + "/" + name
		var exists bool
		if err := pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE filename=$1)", key).Scan(&exists); err != nil {
			return err
		}
		if exists {
			continue
		}

		body, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return err
		}
		if _, err := pool.Exec(ctx, string(body)); err != nil {
			return fmt.Errorf("apply %s %s: %w", kind, name, err)
		}
		if _, err := pool.Exec(ctx, "INSERT INTO schema_migrations(filename) VALUES ($1)", key); err != nil {
			return err
		}
		fmt.Printf("applied %s: %s\n", kind, name)
	}
	return nil
}
