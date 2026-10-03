-- Profile edit page: the extra fields a user can fill in, and uploaded
-- avatars. Avatars are small (resized in the browser, capped server-side),
-- so they live in Postgres rather than needing object storage.
ALTER TABLE users ADD COLUMN IF NOT EXISTS gender TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS birthday DATE;
ALTER TABLE users ADD COLUMN IF NOT EXISTS website_url TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS github_url TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS linkedin_url TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS x_url TEXT;

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'users_gender_check') THEN
    ALTER TABLE users ADD CONSTRAINT users_gender_check
      CHECK (gender IN ('male', 'female', 'non_binary', 'other'));
  END IF;
END $$;

-- One row per user with an uploaded avatar. users.avatar_url points at the
-- serving endpoint (with a version query) while a row exists.
CREATE TABLE IF NOT EXISTS user_avatars (
    user_id      UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    content_type TEXT NOT NULL,
    data         BYTEA NOT NULL,
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
