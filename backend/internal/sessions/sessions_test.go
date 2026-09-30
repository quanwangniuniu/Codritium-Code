package sessions

import (
	"context"
	"net/http"
	"testing"

	"codritium/backend/internal/auth"
	"codritium/backend/internal/platform/testutil"
	"codritium/backend/internal/problems"
)

func handler(t *testing.T) (Handler, string) {
	t.Helper()
	pool := testutil.Pool(t)
	var slug string
	if err := pool.QueryRow(context.Background(),
		`SELECT slug FROM problems WHERE status = 'published' ORDER BY slug LIMIT 1`).Scan(&slug); err != nil {
		t.Skipf("needs a seeded problem: %v", err)
	}
	return Handler{Store: Store{Pool: pool}, Problems: problems.Store{Pool: pool}}, slug
}

func start(t *testing.T, h Handler, u *auth.User, body string) (*startResponse, int) {
	t.Helper()
	rr := testutil.Call(t, http.HandlerFunc(h.start), "POST", "/api/sessions", nil, u, body)
	if rr.Code != http.StatusOK {
		return nil, rr.Code
	}
	resp := testutil.Decode[startResponse](t, rr)
	return &resp, rr.Code
}

func TestStart_Validation(t *testing.T) {
	h, _ := handler(t)
	if _, code := start(t, h, nil, `{}`); code != http.StatusUnauthorized {
		t.Fatalf("anon: %d", code)
	}
	u := testutil.NewUser(t, h.Store.Pool, "user")
	if _, code := start(t, h, u, `{}`); code != http.StatusBadRequest {
		t.Fatalf("missing slug: %d", code)
	}
	if _, code := start(t, h, u, `{"challenge_slug":"does-not-exist"}`); code != http.StatusNotFound {
		t.Fatalf("unknown slug: %d", code)
	}
}

func TestStart_ReuseForceNewAndIsolation(t *testing.T) {
	h, slug := handler(t)
	alice := testutil.NewUser(t, h.Store.Pool, "user")
	bob := testutil.NewUser(t, h.Store.Pool, "user")
	body := `{"challenge_slug":"` + slug + `"}`

	a1, _ := start(t, h, alice, body)
	a2, _ := start(t, h, alice, body)
	if !a1.Created || a2.Created || a1.SessionID != a2.SessionID {
		t.Fatalf("reuse: first=%+v second=%+v", a1, a2)
	}
	a3, _ := start(t, h, alice, `{"challenge_slug":"`+slug+`","force_new":true}`)
	if !a3.Created || a3.SessionID == a1.SessionID {
		t.Fatalf("force_new: %+v", a3)
	}
	b1, _ := start(t, h, bob, body)
	if !b1.Created || b1.SessionID == a1.SessionID {
		t.Fatalf("bob reused alice's session: %+v", b1)
	}
}

func TestOwnershipAndTurns(t *testing.T) {
	h, slug := handler(t)
	ctx := context.Background()
	alice := testutil.NewUser(t, h.Store.Pool, "user")
	bob := testutil.NewUser(t, h.Store.Pool, "user")
	id, _, err := h.Store.Create(ctx, alice.ID, slug, "easy")
	if err != nil {
		t.Fatal(err)
	}
	if ok, _ := h.Store.SessionOwnedBy(ctx, id, alice); !ok {
		t.Fatal("owner rejected")
	}
	if ok, _ := h.Store.SessionOwnedBy(ctx, id, bob); ok {
		t.Fatal("other user accepted")
	}
	if n, _ := h.Store.NextTurn(ctx, id); n != 1 {
		t.Fatalf("turn=%d", n)
	}
	if n, _ := h.Store.NextTurn(ctx, id); n != 2 {
		t.Fatalf("turn=%d", n)
	}
	got, err := h.Store.Attempted(ctx, alice.ID)
	if err != nil || len(got) != 1 || got[0].Slug != slug {
		t.Fatalf("attempted=%v err=%v", got, err)
	}
}
