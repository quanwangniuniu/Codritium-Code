-- Codritium 1.0 auth-finalization schema patches per the login-tech impl-spec.
-- oauth_states gains code_verifier so the GitHub flow can store its PKCE
-- verifier; Google rows leave the column NULL. magic_link_tokens gains
-- created_at so the 60-second rate limit on /api/auth/email/request can
-- query MAX(created_at) per email. Indexes follow login-tech §7.

ALTER TABLE oauth_states
    ADD COLUMN IF NOT EXISTS code_verifier TEXT;

CREATE INDEX IF NOT EXISTS idx_oauth_states_expired
    ON oauth_states(expires_at);

ALTER TABLE magic_link_tokens
    ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

CREATE INDEX IF NOT EXISTS idx_magic_link_email_created
    ON magic_link_tokens(email, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_magic_link_expired
    ON magic_link_tokens(expires_at) WHERE consumed_at IS NULL;
