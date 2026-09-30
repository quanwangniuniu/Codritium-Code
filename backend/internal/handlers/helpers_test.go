package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"codritium/backend/internal/auth"
	"codritium/backend/internal/platform/testutil"
)

// Package-local shorthands over testutil, kept while handlers is split
// into modules.

func newForumUser(t *testing.T, pool *pgxpool.Pool, role string) *auth.User {
	t.Helper()
	return testutil.NewUser(t, pool, role)
}

// forumCall invokes a handler directly. id fills the {id} path value.
func forumCall(t *testing.T, h http.HandlerFunc, method, target, id string, u *auth.User, body string) *httptest.ResponseRecorder {
	t.Helper()
	var pv map[string]string
	if id != "" {
		pv = map[string]string{"id": id}
	}
	return testutil.Call(t, h, method, target, pv, u, body)
}

func wantStatus(t *testing.T, rr *httptest.ResponseRecorder, want int) {
	t.Helper()
	testutil.WantStatus(t, rr, want)
}
