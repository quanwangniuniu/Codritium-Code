package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
)

func onlyCookie(t *testing.T, rec *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	cs := rec.Result().Cookies()
	if len(cs) != 1 {
		t.Fatalf("got %d cookies, want 1", len(cs))
	}
	return cs[0]
}

func TestSessionCookieCarriesDomainAndSecure(t *testing.T) {
	opts := CookieOptions{Domain: "codritium.com", Secure: true}
	id := uuid.New()

	set := httptest.NewRecorder()
	SetSessionCookie(set, id, "secret", opts)
	c := onlyCookie(t, set)
	if !c.Secure || c.Domain != "codritium.com" || !c.HttpOnly || c.MaxAge != cookieMaxAge {
		t.Fatalf("set cookie = %+v", c)
	}
	if got, err := verifyCookie(c.Value, "secret"); err != nil || got != id {
		t.Fatalf("verifyCookie = (%v, %v), want %v", got, err, id)
	}

	// Logout must clear with the same Domain/Secure or the browser keeps
	// the original cookie.
	// It also clears the host-only cookie issued before Domain was set.
	out := httptest.NewRecorder()
	Logout(opts)(out, httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil))
	cs := out.Result().Cookies()
	if len(cs) != 2 {
		t.Fatalf("logout set %d cookies, want 2 (domain + host-only)", len(cs))
	}
	domains := map[string]bool{}
	for _, c := range cs {
		if !c.Secure || c.MaxAge >= 0 || c.Value != "" || c.Name != CookieName {
			t.Fatalf("clear cookie = %+v", c)
		}
		domains[c.Domain] = true
	}
	if !domains["codritium.com"] || !domains[""] {
		t.Fatalf("cleared domains = %v, want codritium.com and host-only", domains)
	}

	// Without a Domain there is only one cookie to clear.
	out = httptest.NewRecorder()
	Logout(CookieOptions{})(out, httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil))
	if n := len(out.Result().Cookies()); n != 1 {
		t.Fatalf("host-only logout set %d cookies, want 1", n)
	}
}

func TestSessionCookieDevDefaults(t *testing.T) {
	rec := httptest.NewRecorder()
	SetSessionCookie(rec, uuid.New(), "secret", CookieOptions{})
	c := onlyCookie(t, rec)
	if c.Secure || c.Domain != "" {
		t.Fatalf("zero CookieOptions should give a host-only non-Secure cookie, got %+v", c)
	}
}
