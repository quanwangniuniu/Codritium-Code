-- Codritium 1.0 schema upgrade: problems classification + user profile fields.
-- Adds company_premium category, tag/status/sample_test columns on problems,
-- and tier/credits/role/avatar_url/bio/email columns on users. Migrates the
-- legacy is_pro boolean to tier='pro' before dropping the column.

-- problems: open category to company_premium
ALTER TABLE problems DROP CONSTRAINT IF EXISTS problems_category_check;
ALTER TABLE problems ADD CONSTRAINT problems_category_check
  CHECK (category IN ('debugging','refactoring','feature_build','security','company_premium'));

-- problems: tags + status + sample_test
ALTER TABLE problems ADD COLUMN IF NOT EXISTS tags TEXT[] NOT NULL DEFAULT '{}';
ALTER TABLE problems ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'published'
  CHECK (status IN ('draft','published','disabled'));
ALTER TABLE problems ADD COLUMN IF NOT EXISTS sample_test_filename TEXT;
ALTER TABLE problems ADD COLUMN IF NOT EXISTS sample_test_content TEXT;

CREATE INDEX IF NOT EXISTS idx_problems_status ON problems(status);
CREATE INDEX IF NOT EXISTS idx_problems_tags ON problems USING GIN(tags);

-- users: profile fields for three-way login (Google / GitHub / Email)
ALTER TABLE users ADD COLUMN IF NOT EXISTS email TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS avatar_url TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS bio TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS tier TEXT NOT NULL DEFAULT 'standard'
  CHECK (tier IN ('standard','pro','max'));
ALTER TABLE users ADD COLUMN IF NOT EXISTS credits INT NOT NULL DEFAULT 0;
ALTER TABLE users ADD COLUMN IF NOT EXISTS role TEXT NOT NULL DEFAULT 'user'
  CHECK (role IN ('user','admin'));

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email ON users(email) WHERE email IS NOT NULL;

-- Migrate legacy is_pro boolean into tier, then drop the column.
UPDATE users SET tier = 'pro' WHERE is_pro = TRUE;
ALTER TABLE users DROP COLUMN IF EXISTS is_pro;
