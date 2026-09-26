-- solutions_replies: stores authoritative AI-generated walkthroughs and any
-- shared user replays. Each row holds a JSON array of ReplyEnvelope objects
-- (same envelope shape as session_events, transformed for the replay UI).
-- Per 2026-05-20 product decision, retention is unbounded for the MVP:
-- expiry_at remains nullable so a future tier-based cleanup can be added
-- without a schema change, but no current code reads it.

CREATE TABLE IF NOT EXISTS solutions_replies (
    id              UUID            PRIMARY KEY DEFAULT gen_random_uuid(),
    challenge_id    TEXT            NOT NULL,                     -- joins problems.slug
    source          TEXT            NOT NULL CHECK (source IN ('official_ai', 'user_share')),
    envelopes       JSONB           NOT NULL,                     -- ReplyEnvelope[]
    created_at      TIMESTAMPTZ     NOT NULL DEFAULT now(),
    expiry_at       TIMESTAMPTZ,                                  -- MVP: always NULL, retained for future tier-based retention
    generator_model TEXT,                                         -- official_ai only, e.g. "gemini-2.5-flash"
    candidate_index INT                                           -- official_ai only, when multiple candidates are kept
);

CREATE INDEX IF NOT EXISTS idx_solutions_replies_challenge
    ON solutions_replies(challenge_id);
CREATE INDEX IF NOT EXISTS idx_solutions_replies_source
    ON solutions_replies(source);
