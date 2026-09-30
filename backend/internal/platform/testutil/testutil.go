// Package testutil holds helpers shared by tests across modules: a
// Postgres pool (skipping when none is reachable), user fixtures, and a
// handler-call helper.
package testutil

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"codritium/backend/internal/auth"
)

// DefaultDSN matches docker-compose's Postgres.
const DefaultDSN = "postgres://codritium:codritium@localhost:5434/codritium?sslmode=disable"

var (
	poolOnce sync.Once
	pool     *pgxpool.Pool
	poolErr  error
)

// Pool returns a shared pool to DATABASE_URL (or DefaultDSN), skipping the
// test when Postgres is unreachable. The pool lives for the whole test
// binary, so tests must not close it.
func Pool(t testing.TB) *pgxpool.Pool {
	t.Helper()
	poolOnce.Do(func() {
		dsn := os.Getenv("DATABASE_URL")
		if dsn == "" {
			dsn = DefaultDSN
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		pool, poolErr = pgxpool.New(ctx, dsn)
		if poolErr == nil {
			poolErr = pool.Ping(ctx)
		}
	})
	if poolErr != nil {
		t.Skipf("postgres unavailable: %v", poolErr)
	}
	return pool
}

// NewUser inserts a users row and deletes it (cascading to everything it
// owns) when the test ends.
func NewUser(t testing.TB, pool *pgxpool.Pool, role string) *auth.User {
	t.Helper()
	u := &auth.User{ID: uuid.New(), Role: role}
	u.Handle = "test_" + u.ID.String()[:8]
	u.DisplayName = u.Handle
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO users (id, handle, display_name, role) VALUES ($1, $2, $3, $4)`,
		u.ID, u.Handle, u.DisplayName, role); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, u.ID)
	})
	return u
}

// Call invokes h directly as user u (nil for anonymous). pathValues fills
// {name} segments.
func Call(t testing.TB, h http.Handler, method, target string, pathValues map[string]string, u *auth.User, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	for k, v := range pathValues {
		req.SetPathValue(k, v)
	}
	if u != nil {
		req = req.WithContext(auth.WithUser(req.Context(), u))
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

// Decode unmarshals a recorder's JSON body.
func Decode[T any](t testing.TB, rr *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rr.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode %q: %v", rr.Body.String(), err)
	}
	return v
}

// WantStatus fails the test when the recorder's status differs.
func WantStatus(t testing.TB, rr *httptest.ResponseRecorder, want int) {
	t.Helper()
	if rr.Code != want {
		t.Fatalf("status=%d want %d; body=%s", rr.Code, want, rr.Body.String())
	}
}
