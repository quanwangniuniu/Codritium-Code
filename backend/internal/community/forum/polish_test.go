package forum

import (
	"bytes"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"codritium/backend/internal/auth"
	"codritium/backend/internal/platform/testutil"
)

func tinyPNG(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestForum_ImageUploadAndServe(t *testing.T) {
	pool := testutil.Pool(t)
	deps := Handler{Pool: pool}
	alice := testutil.NewUser(t, pool, "user")
	upload := func(u *auth.User, body []byte) *httptest.ResponseRecorder {
		return forumCall(t, http.HandlerFunc(deps.uploadForumImage), http.MethodPost, "/", "", u, string(body))
	}

	testutil.WantStatus(t, upload(nil, tinyPNG(t)), http.StatusUnauthorized)
	testutil.WantStatus(t, upload(alice, []byte("<svg onload=alert(1)>")), http.StatusBadRequest)
	testutil.WantStatus(t, upload(alice, bytes.Repeat([]byte{0}, maxImageBytes+1)), http.StatusRequestEntityTooLarge)

	rr := upload(alice, tinyPNG(t))
	testutil.WantStatus(t, rr, http.StatusCreated)
	got := decodeForum[struct{ ID, URL string }](t, rr)
	if got.URL != "/api/forum/images/"+got.ID {
		t.Fatalf("upload: %+v", got)
	}

	serve := forumCall(t, http.HandlerFunc(deps.getForumImage), http.MethodGet, "/", got.ID, nil, "")
	testutil.WantStatus(t, serve, http.StatusOK)
	if ct := serve.Header().Get("Content-Type"); ct != "image/png" {
		t.Fatalf("content type %q", ct)
	}
	if !strings.Contains(serve.Header().Get("Cache-Control"), "immutable") || serve.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("headers: %v", serve.Header())
	}
	if !bytes.Equal(serve.Body.Bytes(), tinyPNG(t)) {
		t.Fatal("served bytes differ")
	}
	testutil.WantStatus(t, forumCall(t, http.HandlerFunc(deps.getForumImage), http.MethodGet, "/", "not-a-uuid", nil, ""), http.StatusNotFound)
}

func TestForum_PostRevisions(t *testing.T) {
	pool := testutil.Pool(t)
	deps := Handler{Pool: pool}
	alice := testutil.NewUser(t, pool, "user")
	bob := testutil.NewUser(t, pool, "user")
	admin := testutil.NewUser(t, pool, "admin")
	p := createForumPost(t, deps, alice, `{"section":"career","title":"First title","body_md":"v1"}`)
	pid := p.ID.String()
	edit := func(body string) {
		testutil.WantStatus(t, forumCall(t, http.HandlerFunc(deps.updateForumPost), http.MethodPut, "/", pid, alice, body), http.StatusOK)
	}
	edit(`{"section":"career","title":"First title","body_md":"v2"}`)
	edit(`{"section":"general","title":"First title","body_md":"v2"}`) // section only: no new revision
	edit(`{"section":"general","title":"Second title","body_md":"v3"}`)

	revs := func(u *auth.User) *httptest.ResponseRecorder {
		return forumCall(t, http.HandlerFunc(deps.listForumPostRevisions), http.MethodGet, "/", pid, u, "")
	}
	got := decodeForum[struct{ Revisions []forumRevision }](t, revs(alice))
	if len(got.Revisions) != 2 || got.Revisions[0].BodyMD != "v2" || got.Revisions[1].BodyMD != "v1" ||
		got.Revisions[1].Title != "First title" || !got.Revisions[1].WrittenAt.Equal(p.CreatedAt) {
		t.Fatalf("revisions: %+v", got.Revisions)
	}
	testutil.WantStatus(t, revs(admin), http.StatusOK)
	testutil.WantStatus(t, revs(bob), http.StatusNotFound)
	testutil.WantStatus(t, revs(nil), http.StatusUnauthorized)
}

func TestForum_CountNewComments(t *testing.T) {
	pool := testutil.Pool(t)
	deps := Handler{Pool: pool}
	alice := testutil.NewUser(t, pool, "user")
	bob := testutil.NewUser(t, pool, "user")
	p := createForumPost(t, deps, alice, `{"section":"career","title":"Live thread","body_md":"x"}`)
	pid := p.ID.String()

	list := decodeForum[struct {
		AsOf string `json:"as_of"`
	}](t, forumCall(t, http.HandlerFunc(deps.listForumComments), http.MethodGet, "/", pid, alice, ""))
	if _, err := time.Parse(time.RFC3339Nano, list.AsOf); err != nil {
		t.Fatalf("as_of %q: %v", list.AsOf, err)
	}
	for _, u := range []*auth.User{bob, bob, alice} {
		testutil.WantStatus(t, forumCall(t, http.HandlerFunc(deps.createForumComment), http.MethodPost, "/", pid, u, `{"body":"hi"}`), http.StatusCreated)
	}
	count := func(u *auth.User) int {
		rr := forumCall(t, http.HandlerFunc(deps.countNewForumComments), http.MethodGet, "/?since="+url.QueryEscape(list.AsOf), pid, u, "")
		testutil.WantStatus(t, rr, http.StatusOK)
		return decodeForum[struct{ Count int }](t, rr).Count
	}
	// Alice doesn't count her own comment; a signed-out reader sees all three.
	if got := count(alice); got != 2 {
		t.Fatalf("alice new=%d", got)
	}
	if got := count(nil); got != 3 {
		t.Fatalf("anon new=%d", got)
	}
	testutil.WantStatus(t, forumCall(t, http.HandlerFunc(deps.countNewForumComments), http.MethodGet, "/?since=nope", pid, nil, ""), http.StatusBadRequest)
}
