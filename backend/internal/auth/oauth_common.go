package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"golang.org/x/oauth2"
)

// OAuthStateExpiry caps how long a /start request can sit before the user
// completes /callback. GitHub authorization codes expire in 10 minutes so we
// align here; Google is more generous but matching the tighter bound is
// fine.
const OAuthStateExpiry = 10 * time.Minute

// MagicLinkTTL is the lifetime of a magic-link token. Per login-tech §5
// Q5=A this is 24 hours and the token is one-use.
const MagicLinkTTL = 24 * time.Hour

// MagicLinkRateLimit is the minimum gap between successive magic-link
// requests for the same email address.
const MagicLinkRateLimit = 60 * time.Second

// Provider names the identity provider behind a session. The email
// "provider" is the magic-link path.
type Provider string

const (
	ProviderGoogle Provider = "google"
	ProviderGitHub Provider = "github"
	ProviderEmail  Provider = "email"
)

// ErrInvalidState fires when a state token doesn't exist, expired, or was
// presented to the wrong provider.
var ErrInvalidState = errors.New("oauth state invalid or expired")

// GenerateState returns 32 random bytes as a hex string. 64 chars of
// hex is unguessable per the OAuth 2.0 CSRF guidance.
func GenerateState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// GeneratePKCEVerifier wraps oauth2.GenerateVerifier so callers don't have
// to pull in the oauth2 package just for the PKCE start path.
func GeneratePKCEVerifier() string {
	return oauth2.GenerateVerifier()
}

// OAuthStateStore persists short-lived state values for the OAuth round
// trip. Insert stores a row with the given provider and (for GitHub PKCE)
// code verifier; Consume returns the stored verifier and removes the row
// in one atomic operation.
type OAuthStateStore interface {
	Insert(ctx context.Context, state string, provider Provider, codeVerifier string, ttl time.Duration) error
	Consume(ctx context.Context, state string, provider Provider) (codeVerifier string, ok bool, err error)
}

// MagicLinkTokenStore handles the email magic-link tokens. Insert stores a
// new token with explicit expiry; Consume returns the email and marks the
// token spent in one statement; LastRequestedAt powers the 60-second rate
// limit on /api/auth/email/request.
type MagicLinkTokenStore interface {
	Insert(ctx context.Context, token, email string, expiresAt time.Time) error
	Consume(ctx context.Context, token string) (email string, ok bool, err error)
	LastRequestedAt(ctx context.Context, email string) (time.Time, error)
}

// UserStore upserts users from the three login paths and links the
// provider record. UpsertFromOAuth covers Google + GitHub (whose providers
// supply name/picture and a stable provider_user_id); UpsertFromEmail
// covers the magic-link path where only the email is known.
type UserStore interface {
	UpsertFromOAuth(ctx context.Context, email, displayName, avatarURL string, provider Provider, providerUserID string) (uuid.UUID, error)
	UpsertFromEmail(ctx context.Context, email string) (uuid.UUID, error)
}

// SessionManager bridges the per-handler "create session for user" call to
// whatever cookie + signing implementation the package uses. The concrete
// adapter in store.go wraps SetSessionCookie.
type SessionManager interface {
	Create(w http.ResponseWriter, userID uuid.UUID, ttl time.Duration) error
}
