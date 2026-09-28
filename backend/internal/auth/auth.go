package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ctxKey string

const userCtxKey ctxKey = "user"

// User mirrors the row in users plus a couple of derived fields. IsPro is
// computed from tier so older frontend code that branches on a boolean keeps
// working while the tier-aware UI ships.
type User struct {
	ID          uuid.UUID
	Handle      string
	Email       string
	DisplayName string
	Region      string
	Tier        string
	Credits     int
	Role        string
	AvatarURL   string
	AvatarColor string
	Bio         string
	IsPro       bool
}

// CookieName carries an HMAC-signed user id. Format: "<user_id>.<sig>" where
// sig = base64url(HMAC-SHA256(user_id, COOKIE_SECRET)). Cookies that don't
// parse, don't verify, or reference a missing user are silently dropped.
const CookieName = "codritium_session"

const cookieMaxAge = 86400 * 30

// Middleware loads the user identified by the signed session cookie and
// injects it into the request context. A missing, malformed, or tampered
// cookie leaves the user as nil; downstream handlers decide whether that is
// acceptable (typically by returning 401).
func Middleware(pool *pgxpool.Pool, cookieSecret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			if c, err := r.Cookie(CookieName); err == nil && c.Value != "" {
				if id, err := verifyCookie(c.Value, cookieSecret); err == nil {
					if u, err := loadUserByID(r.Context(), pool, id); err == nil {
						ctx = context.WithValue(ctx, userCtxKey, u)
					}
				}
			}
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// FromContext returns the user injected by Middleware, or nil if none.
func FromContext(ctx context.Context) *User {
	u, _ := ctx.Value(userCtxKey).(*User)
	return u
}

// WithUser injects a user into the context using the key Middleware uses.
// Exposed so tests can construct a request that FromContext accepts without
// going through the cookie + DB path.
func WithUser(ctx context.Context, u *User) context.Context {
	return context.WithValue(ctx, userCtxKey, u)
}

// CookieOptions carries the per-deployment session cookie attributes
// (COOKIE_DOMAIN / COOKIE_SECURE). The zero value is a host-only,
// non-Secure cookie, which is only appropriate for local HTTP dev.
type CookieOptions struct {
	Domain string
	Secure bool
}

// sessionCookie builds the session cookie with the shared attributes so
// set and clear always agree — a browser only deletes a cookie whose
// Domain/Path match the one it stored.
func sessionCookie(value string, maxAge int, opts CookieOptions) *http.Cookie {
	return &http.Cookie{
		Name:     CookieName,
		Value:    value,
		Path:     "/",
		Domain:   opts.Domain,
		Secure:   opts.Secure,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   maxAge,
	}
}

// SetSessionCookie writes the signed session cookie for the given user.
func SetSessionCookie(w http.ResponseWriter, userID uuid.UUID, cookieSecret string, opts CookieOptions) {
	http.SetCookie(w, sessionCookie(signCookie(userID, cookieSecret), cookieMaxAge, opts))
}

func signCookie(userID uuid.UUID, cookieSecret string) string {
	payload := userID.String()
	mac := hmac.New(sha256.New, []byte(cookieSecret))
	mac.Write([]byte(payload))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return payload + "." + sig
}

func verifyCookie(value, cookieSecret string) (uuid.UUID, error) {
	parts := strings.SplitN(value, ".", 2)
	if len(parts) != 2 {
		return uuid.Nil, errors.New("cookie malformed")
	}
	id, err := uuid.Parse(parts[0])
	if err != nil {
		return uuid.Nil, errors.New("cookie payload not a uuid")
	}
	expected, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return uuid.Nil, errors.New("cookie sig not base64")
	}
	mac := hmac.New(sha256.New, []byte(cookieSecret))
	mac.Write([]byte(parts[0]))
	if !hmac.Equal(expected, mac.Sum(nil)) {
		return uuid.Nil, errors.New("cookie sig mismatch")
	}
	return id, nil
}

const userColumns = `
	id, handle, COALESCE(email,''), display_name, COALESCE(region,''),
	tier, credits, role,
	COALESCE(avatar_url,''), COALESCE(avatar_color,'#7dd3fc'), COALESCE(bio,'')
`

func scanUser(row interface {
	Scan(dest ...any) error
}) (*User, error) {
	u := &User{}
	if err := row.Scan(
		&u.ID, &u.Handle, &u.Email, &u.DisplayName, &u.Region,
		&u.Tier, &u.Credits, &u.Role,
		&u.AvatarURL, &u.AvatarColor, &u.Bio,
	); err != nil {
		return nil, err
	}
	u.IsPro = u.Tier == "pro" || u.Tier == "max"
	return u, nil
}

func loadUserByID(ctx context.Context, pool *pgxpool.Pool, id uuid.UUID) (*User, error) {
	return scanUser(pool.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE id = $1`, id))
}

func loadUserByHandle(ctx context.Context, pool *pgxpool.Pool, handle string) (*User, error) {
	return scanUser(pool.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE handle = $1`, handle))
}

// SwitchUser is the dev-only mock login. It writes a signed cookie for any
// handle present in users — useful for /api/auth/switch?handle=alice during
// local development. Production logins go through GoogleHandler /
// GitHubHandler / EmailHandler in google.go / github.go / email.go; those
// handlers always upsert from a verified provider identity before signing
// a session.
func SwitchUser(pool *pgxpool.Pool, cookieSecret string, opts CookieOptions) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		handle := r.URL.Query().Get("handle")
		if handle == "" {
			http.Error(w, "handle query param required", http.StatusBadRequest)
			return
		}
		u, err := loadUserByHandle(r.Context(), pool, handle)
		if err != nil {
			http.Error(w, "unknown handle", http.StatusUnauthorized)
			return
		}
		SetSessionCookie(w, u.ID, cookieSecret, opts)
		w.WriteHeader(http.StatusNoContent)
	}
}

// Logout clears the session cookie. POST /api/auth/logout
func Logout(opts CookieOptions) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, sessionCookie("", -1, opts))
		w.WriteHeader(http.StatusNoContent)
	}
}
