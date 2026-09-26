-- Add mode column to candidate_sessions for tips-agent gating.
-- Three modes are recognised: practice (the v0.8 path, where the tips-agent
-- is the only one allowed to serve hints), simulator (interview rehearsal,
-- tips off), reply (post-mortem replay, tips off). Default 'practice' so
-- existing rows continue to behave the way they always have.

ALTER TABLE candidate_sessions
    ADD COLUMN IF NOT EXISTS mode TEXT NOT NULL DEFAULT 'practice'
    CHECK (mode IN ('practice', 'simulator', 'reply'));

CREATE INDEX IF NOT EXISTS idx_candidate_sessions_mode
    ON candidate_sessions(mode);
