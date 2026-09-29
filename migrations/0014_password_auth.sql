-- Store only password hashes for local email/password accounts.
ALTER TABLE users ADD COLUMN IF NOT EXISTS password_hash TEXT;

-- OAuth/magic-link accounts and password accounts must resolve the same
-- canonical email regardless of letter casing.
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email_lower
    ON users (lower(email))
    WHERE email IS NOT NULL;