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
	out := httptest.NewRecorder()
	Logout(opts)(out, httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil))
	c = onlyCookie(t, out)
	if !c.Secure || c.Domain != "codritium.com" || c.MaxAge >= 0 || c.Value != "" {
		t.Fatalf("clear cookie = %+v", c)
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
