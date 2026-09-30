package testutil

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"codritium/backend/internal/config"
	"codritium/backend/internal/migrate"
	"codritium/backend/internal/problems/seed"
)

// DefaultTestDSN is a dedicated database next to the dev one, so tests
// never touch (or migrate) the database a running dev server uses.
const DefaultTestDSN = "postgres://codritium:codritium@localhost:5434/codritium_test?sslmode=disable"

// openTestDB connects to TEST_DATABASE_URL (or DefaultTestDSN), creating
// the database if needed, then applies migrations and the problem seed.
func openTestDB() (*pgxpool.Pool, error) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = DefaultTestDSN
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if err := ensureDatabase(ctx, dsn); err != nil {
		return nil, err
	}
	p, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, err
	}
	if err := p.Ping(ctx); err != nil {
		p.Close()
		return nil, err
	}
	paths, err := config.ResolvePaths()
	if err != nil {
		p.Close()
		return nil, err
	}
	if err := migrate.Run(ctx, p, paths.Migrations, paths.Seed); err != nil {
		p.Close()
		return nil, fmt.Errorf("migrate test db: %w", err)
	}
	if err := seed.FromDir(ctx, p, paths.ProblemsSeed); err != nil {
		p.Close()
		return nil, fmt.Errorf("seed test db: %w", err)
	}
	return p, nil
}

// ensureDatabase creates the DSN's database via the server's maintenance
// database when it does not exist yet.
func ensureDatabase(ctx context.Context, dsn string) error {
	u, err := url.Parse(dsn)
	if err != nil {
		return err
	}
	name := strings.TrimPrefix(u.Path, "/")
	admin := *u
	admin.Path = "/postgres"
	conn, err := pgx.Connect(ctx, admin.String())
	if err != nil {
		return err
	}
	defer conn.Close(ctx)
	var exists bool
	if err := conn.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)`, name).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return nil
	}
	_, err = conn.Exec(ctx, `CREATE DATABASE `+pgx.Identifier{name}.Sanitize())
	return err
}
