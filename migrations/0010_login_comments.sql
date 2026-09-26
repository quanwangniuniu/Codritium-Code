-- Codritium 1.0 schema: three-way login (Google / GitHub / Email magic link)
-- plus per-problem comment threads with up/down votes.
-- DDL only; handler implementations land after the login-tech research note.

-- user_oauth_links: one row per (user, provider) binding.
-- provider_user_id holds the canonical id returned by the IdP
-- (Google sub claim, GitHub user id, or the verified email for the magic-link
-- provider).
CREATE TABLE IF NOT EXISTS user_oauth_links (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider TEXT NOT NULL CHECK (provider IN ('google','github','email')),
    provider_user_id TEXT NOT NULL,
    linked_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, provider)
);
CREATE INDEX IF NOT EXISTS idx_oauth_provider_uid
    ON user_oauth_links(provider, provider_user_id);

-- magic_link_tokens: single-use email tokens. token is opaque; consumed_at
-- is set when the user follows the link so a replay attempt fails.
CREATE TABLE IF NOT EXISTS magic_link_tokens (
    token TEXT PRIMARY KEY,
    email TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_magic_link_email_expires
    ON magic_link_tokens(email, expires_at);

-- oauth_states: CSRF protection for OAuth flows; one row per /start request,
-- consumed on /callback. expires_at gives a hard upper bound for sweep.
CREATE TABLE IF NOT EXISTS oauth_states (
    state TEXT PRIMARY KEY,
    provider TEXT NOT NULL CHECK (provider IN ('google','github')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL
);

-- comments: per-problem threaded discussion (parent_id self-reference).
-- Soft delete via deleted_at; vote tallies are denormalized counters
-- maintained by the votes API.
CREATE TABLE IF NOT EXISTS comments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    problem_slug TEXT NOT NULL,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    body TEXT NOT NULL,
    upvotes INT NOT NULL DEFAULT 0,
    downvotes INT NOT NULL DEFAULT 0,
    parent_id UUID REFERENCES comments(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_comments_problem_active
    ON comments(problem_slug, created_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_comments_user
    ON comments(user_id);

-- comment_votes: one row per (comment, user). value is -1 or 1; flipping
-- requires UPDATE rather than INSERT to keep PRIMARY KEY semantics.
CREATE TABLE IF NOT EXISTS comment_votes (
    comment_id UUID NOT NULL REFERENCES comments(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    value INT NOT NULL CHECK (value IN (-1, 1)),
    voted_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (comment_id, user_id)
);
