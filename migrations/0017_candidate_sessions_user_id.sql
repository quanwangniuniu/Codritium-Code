-- candidate_sessions identified candidates by handle (candidate_id TEXT,
-- migration 0002), so renaming a handle orphaned every session and there
-- was no foreign key. Key sessions by users.id like every other table.
ALTER TABLE candidate_sessions ADD COLUMN IF NOT EXISTS user_id UUID REFERENCES users(id) ON DELETE CASCADE;

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM information_schema.columns
             WHERE table_name = 'candidate_sessions' AND column_name = 'candidate_id') THEN
    UPDATE candidate_sessions cs SET user_id = u.id
    FROM users u WHERE cs.user_id IS NULL AND u.handle = cs.candidate_id;
    -- Sessions whose user no longer exists would have cascaded away with a
    -- foreign key; remove them now so user_id can be NOT NULL.
    DELETE FROM candidate_sessions WHERE user_id IS NULL;
    DROP INDEX IF EXISTS idx_candidate_sessions_candidate;
    ALTER TABLE candidate_sessions DROP COLUMN candidate_id;
  END IF;
END $$;

ALTER TABLE candidate_sessions ALTER COLUMN user_id SET NOT NULL;
CREATE INDEX IF NOT EXISTS idx_candidate_sessions_user_started
    ON candidate_sessions(user_id, started_at DESC);
