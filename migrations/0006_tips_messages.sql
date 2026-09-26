-- tips_messages: persistent tutor conversation, isolated from session_events.
-- The grader reads session_events; tips conversations must NOT inform scoring,
-- so they live in their own table. One row per turn; seq is the monotonic
-- per-session index assigned at write time. Practice mode is the only mode
-- that writes here — simulator and reply sessions never invoke the tips API
-- and therefore never accumulate tips_messages rows.

CREATE TABLE IF NOT EXISTS tips_messages (
    session_id  UUID            NOT NULL REFERENCES candidate_sessions(session_id) ON DELETE CASCADE,
    seq         INTEGER         NOT NULL,
    role        TEXT            NOT NULL CHECK (role IN ('user', 'model')),
    text        TEXT            NOT NULL,
    created_at  TIMESTAMPTZ     NOT NULL DEFAULT now(),

    PRIMARY KEY (session_id, seq)
);

CREATE INDEX IF NOT EXISTS idx_tips_messages_session
    ON tips_messages(session_id, seq);
