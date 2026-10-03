package account

import (
	"bytes"
	"image"
	"image/png"
	"net/http"
	"strings"
	"testing"
	"time"

	"codritium/backend/internal/platform/testutil"
)

func TestNormalizeLink(t *testing.T) {
	cases := []struct {
		kind linkKind
		in   string
		want string
		ok   bool
	}{
		{linkWebsite, "", "", true},
		{linkWebsite, "  example.com/me ", "https://example.com/me", true},
		{linkWebsite, "http://example.com", "http://example.com", true},
		{linkWebsite, "javascript:alert(1)", "", false},
		{linkWebsite, "ftp://example.com", "", false},
		{linkWebsite, "https://user:pw@example.com", "", false},
		{linkWebsite, "localhost", "", false},
		{linkWebsite, "https://example.com/" + strings.Repeat("a", maxURLBytes), "", false},
		{linkGithub, "octocat", "https://github.com/octocat", true},
		{linkGithub, "@octocat", "https://github.com/octocat", true},
		{linkGithub, "github.com/octocat", "https://github.com/octocat", true},
		{linkGithub, "https://www.github.com/octocat", "https://www.github.com/octocat", true},
		{linkGithub, "https://github.com", "", false},
		{linkGithub, "https://evil.com/github.com/octocat", "", false},
		{linkGithub, "https://notgithub.com/octocat", "", false},
		{linkLinkedin, "ada-lovelace", "https://www.linkedin.com/in/ada-lovelace", true},
		{linkLinkedin, "https://au.linkedin.com/in/ada", "https://au.linkedin.com/in/ada", true},
		{linkX, "@ada", "https://x.com/ada", true},
		{linkX, "https://twitter.com/ada", "https://twitter.com/ada", true},
		{linkX, "https://example.com/ada", "", false},
	}
	for _, c := range cases {
		got, ok := normalizeLink(c.kind, c.in)
		if got != c.want || ok != c.ok {
			t.Errorf("normalizeLink(%q) = %q, %v; want %q, %v", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestApplyDetailsUpdate(t *testing.T) {
	str := func(s string) *string { return &s }
	now := time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name    string
		req     updateMeRequest
		wantErr string
	}{
		{"gender ok", updateMeRequest{Gender: str("non_binary")}, ""},
		{"gender cleared", updateMeRequest{Gender: str("")}, ""},
		{"gender unknown", updateMeRequest{Gender: str("robot")}, "invalid_gender"},
		{"birthday ok", updateMeRequest{Birthday: str("1990-02-28")}, ""},
		{"birthday cleared", updateMeRequest{Birthday: str("")}, ""},
		{"birthday not a date", updateMeRequest{Birthday: str("1990-02-30")}, "invalid_birthday"},
		{"birthday in future", updateMeRequest{Birthday: str("2026-10-04")}, "invalid_birthday"},
		{"birthday too old", updateMeRequest{Birthday: str("1899-12-31")}, "invalid_birthday"},
		{"bad x link", updateMeRequest{XURL: str("https://example.com/a")}, "invalid_x_url"},
	}
	for _, c := range cases {
		d := details{Gender: "male", Birthday: "2000-01-01"}
		err := applyDetailsUpdate(&d, c.req, now)
		got := ""
		if err != nil {
			got = err.Error()
		}
		if got != c.wantErr {
			t.Errorf("%s: err=%q want %q", c.name, got, c.wantErr)
		}
	}
	d := details{Gender: "male", GithubURL: "https://github.com/old"}
	_ = applyDetailsUpdate(&d, updateMeRequest{GithubURL: str("ada")}, now)
	if d.Gender != "male" || d.GithubURL != "https://github.com/ada" {
		t.Errorf("partial update changed wrong fields: %+v", d)
	}
}

func TestUpdateMe_PersistsDetails(t *testing.T) {
	pool := testutil.Pool(t)
	u := testutil.NewUser(t, pool, "user")
	h := Handler{Pool: pool}

	rr := testutil.Call(t, http.HandlerFunc(h.updateMe), http.MethodPatch, "/api/me", nil, u,
		`{"gender":"female","birthday":"1990-05-17","website_url":"example.com","github_url":"ada"}`)
	testutil.WantStatus(t, rr, http.StatusOK)

	// A later single-field save must not wipe the others.
	rr = testutil.Call(t, http.HandlerFunc(h.updateMe), http.MethodPatch, "/api/me", nil, u, `{"x_url":"@ada"}`)
	testutil.WantStatus(t, rr, http.StatusOK)

	rr = testutil.Call(t, http.HandlerFunc(h.me), http.MethodGet, "/api/me", nil, u, "")
	testutil.WantStatus(t, rr, http.StatusOK)
	got := testutil.Decode[map[string]any](t, rr)
	want := map[string]string{
		"gender": "female", "birthday": "1990-05-17", "website_url": "https://example.com",
		"github_url": "https://github.com/ada", "linkedin_url": "", "x_url": "https://x.com/ada",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %v, want %q", k, got[k], v)
		}
	}

	rr = testutil.Call(t, http.HandlerFunc(h.updateMe), http.MethodPatch, "/api/me", nil, u, `{"birthday":""}`)
	testutil.WantStatus(t, rr, http.StatusOK)
	if b := testutil.Decode[map[string]any](t, rr)["birthday"]; b != "" {
		t.Errorf("birthday not cleared: %v", b)
	}
}

func TestAvatar_UploadServeDelete(t *testing.T) {
	pool := testutil.Pool(t)
	u := testutil.NewUser(t, pool, "user")
	h := Handler{Pool: pool}
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 4, 4))); err != nil {
		t.Fatal(err)
	}
	img := buf.String()
	id := map[string]string{"id": u.ID.String()}
	get := func() int {
		return testutil.Call(t, http.HandlerFunc(h.getAvatar), http.MethodGet, "/api/users/x/avatar", id, nil, "").Code
	}

	if code := get(); code != http.StatusNotFound {
		t.Fatalf("avatar before upload: %d", code)
	}

	rr := testutil.Call(t, http.HandlerFunc(h.putAvatar), http.MethodPut, "/api/me/avatar", nil, u, "<svg onload=alert(1)>")
	testutil.WantStatus(t, rr, http.StatusBadRequest)
	rr = testutil.Call(t, http.HandlerFunc(h.putAvatar), http.MethodPut, "/api/me/avatar", nil, u,
		img+strings.Repeat("x", maxAvatarBytes))
	testutil.WantStatus(t, rr, http.StatusRequestEntityTooLarge)
	rr = testutil.Call(t, http.HandlerFunc(h.putAvatar), http.MethodPut, "/api/me/avatar", nil, nil, img)
	testutil.WantStatus(t, rr, http.StatusUnauthorized)

	rr = testutil.Call(t, http.HandlerFunc(h.putAvatar), http.MethodPut, "/api/me/avatar", nil, u, img)
	testutil.WantStatus(t, rr, http.StatusOK)
	avatarURL, _ := testutil.Decode[map[string]any](t, rr)["avatar_url"].(string)
	if !strings.HasPrefix(avatarURL, "/api/users/"+u.ID.String()+"/avatar?v=") {
		t.Fatalf("avatar_url = %q", avatarURL)
	}

	rr = testutil.Call(t, http.HandlerFunc(h.getAvatar), http.MethodGet, "/api/users/x/avatar", id, nil, "")
	testutil.WantStatus(t, rr, http.StatusOK)
	if rr.Header().Get("Content-Type") != "image/png" || rr.Body.String() != img {
		t.Errorf("served avatar differs: type %q, %d bytes", rr.Header().Get("Content-Type"), rr.Body.Len())
	}

	rr = testutil.Call(t, http.HandlerFunc(h.deleteAvatar), http.MethodDelete, "/api/me/avatar", nil, u, "")
	testutil.WantStatus(t, rr, http.StatusOK)
	if got := testutil.Decode[map[string]any](t, rr)["avatar_url"]; got != "" {
		t.Errorf("avatar_url after delete = %v", got)
	}
	if code := get(); code != http.StatusNotFound {
		t.Errorf("avatar after delete: %d", code)
	}
}
