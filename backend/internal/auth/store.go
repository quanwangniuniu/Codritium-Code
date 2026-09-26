package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SQLOAuthStateStore implements OAuthStateStore against the oauth_states
// table. The Consume path runs a single DELETE … RETURNING so a state can
// only ever be redeemed once.
type SQLOAuthStateStore struct {
	Pool *pgxpool.Pool
}

func NewSQLOAuthStateStore(pool *pgxpool.Pool) *SQLOAuthStateStore {
	return &SQLOAuthStateStore{Pool: pool}
}

func (s *SQLOAuthStateStore) Insert(ctx context.Context, state string, provider Provider, codeVerifier string, ttl time.Duration) error {
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO oauth_states (state, provider, code_verifier, expires_at)
		VALUES ($1, $2, NULLIF($3, ''), NOW() + $4::interval)`,
		state, string(provider), codeVerifier, ttl.String(),
	)
	return err
}

func (s *SQLOAuthStateStore) Consume(ctx context.Context, state string, provider Provider) (string, bool, error) {
	var verifier *string
	err := s.Pool.QueryRow(ctx, `
		DELETE FROM oauth_states
		WHERE state = $1 AND provider = $2 AND expires_at > NOW()
		RETURNING code_verifier`,
		state, string(provider),
	).Scan(&verifier)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	if verifier == nil {
		return "", true, nil
	}
	return *verifier, true, nil
}

// SQLMagicLinkTokenStore implements MagicLinkTokenStore against the
// magic_link_tokens table.
type SQLMagicLinkTokenStore struct {
	Pool *pgxpool.Pool
}

func NewSQLMagicLinkTokenStore(pool *pgxpool.Pool) *SQLMagicLinkTokenStore {
	return &SQLMagicLinkTokenStore{Pool: pool}
}

func (s *SQLMagicLinkTokenStore) Insert(ctx context.Context, token, email string, expiresAt time.Time) error {
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO magic_link_tokens (token, email, expires_at)
		VALUES ($1, $2, $3)`,
		token, email, expiresAt,
	)
	return err
}

func (s *SQLMagicLinkTokenStore) Consume(ctx context.Context, token string) (string, bool, error) {
	var email string
	err := s.Pool.QueryRow(ctx, `
		UPDATE magic_link_tokens
		SET consumed_at = NOW()
		WHERE token = $1 AND expires_at > NOW() AND consumed_at IS NULL
		RETURNING email`,
		token,
	).Scan(&email)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return email, true, nil
}

func (s *SQLMagicLinkTokenStore) LastRequestedAt(ctx context.Context, email string) (time.Time, error) {
	var last *time.Time
	err := s.Pool.QueryRow(ctx,
		`SELECT MAX(created_at) FROM magic_link_tokens WHERE email = $1`,
		email,
	).Scan(&last)
	if err != nil {
		return time.Time{}, err
	}
	if last == nil {
		return time.Time{}, nil
	}
	return *last, nil
}

// SQLUserStore implements UserStore. UpsertFromOAuth resolves the user by
// canonical email (the magic-link path uses the same email column), creates
// the row when absent, and idempotently links the provider record.
type SQLUserStore struct {
	Pool *pgxpool.Pool
}

func NewSQLUserStore(pool *pgxpool.Pool) *SQLUserStore {
	return &SQLUserStore{Pool: pool}
}

func (s *SQLUserStore) UpsertFromOAuth(ctx context.Context, email, displayName, avatarURL string, provider Provider, providerUserID string) (uuid.UUID, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return uuid.Nil, errors.New("user upsert: email required")
	}
	if providerUserID == "" {
		return uuid.Nil, errors.New("user upsert: provider_user_id required")
	}
	display := strings.TrimSpace(displayName)
	if display == "" {
		display = email
	}
	return s.upsertCore(ctx, email, display, avatarURL, provider, providerUserID)
}

func (s *SQLUserStore) UpsertFromEmail(ctx context.Context, email string) (uuid.UUID, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return uuid.Nil, errors.New("user upsert: email required")
	}
	return s.upsertCore(ctx, email, email, "", ProviderEmail, email)
}

func (s *SQLUserStore) upsertCore(ctx context.Context, email, display, avatarURL string, provider Provider, providerUserID string) (uuid.UUID, error) {
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return uuid.Nil, err
	}
	defer tx.Rollback(ctx)

	var userID uuid.UUID
	err = tx.QueryRow(ctx,
		`SELECT id FROM users WHERE email = $1`,
		email,
	).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		// Fresh user. handle has a UNIQUE NOT NULL constraint and isn't
		// surfaced anywhere meaningful for OAuth/email logins, so we mint a
		// random one. Future renames go through a separate flow.
		handle := "u_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:10]
		err = tx.QueryRow(ctx, `
			INSERT INTO users (handle, display_name, email, avatar_url)
			VALUES ($1, $2, $3, NULLIF($4, ''))
			RETURNING id`,
			handle, display, email, avatarURL,
		).Scan(&userID)
		if err != nil {
			return uuid.Nil, err
		}
	} else if err != nil {
		return uuid.Nil, err
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO user_oauth_links (user_id, provider, provider_user_id)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id, provider) DO UPDATE SET provider_user_id = EXCLUDED.provider_user_id`,
		userID, string(provider), providerUserID,
	); err != nil {
		return uuid.Nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, err
	}
	return userID, nil
}

// CookieSessionManager bridges OAuth/magic-link handlers to the existing
// SetSessionCookie path. ttl is honored only via the cookie MaxAge already
// hard-coded in SetSessionCookie; values longer than that are clamped.
type CookieSessionManager struct {
	Secret string
}

func NewCookieSessionManager(secret string) *CookieSessionManager {
	return &CookieSessionManager{Secret: secret}
}

func (m *CookieSessionManager) Create(w http.ResponseWriter, userID uuid.UUID, ttl time.Duration) error {
	_ = ttl // cookie MaxAge is set inside SetSessionCookie; ttl is informational
	SetSessionCookie(w, userID, m.Secret)
	return nil
}
