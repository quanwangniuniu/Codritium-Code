-- v0.8 candidate-agent migration
--
-- Adds four tables that back the v0.8 candidate-agent rewrite:
--   1. candidate_sessions               — one row per attempt (~one problem)
--   2. session_events                   — structured event stream (grader's primary input)
--   3. session_messages                 — chat transcript (audit + Communication dimension)
--   4. session_candidate_corrections    — per-session feedback memory (no cross-session)
--
-- Design invariants (see PLAN/v0.8/design/events_payload_spec.md §7):
--   - seq is per-session monotonic. Do NOT use timestamps for ordering.
--   - session_messages.seq and session_events.seq share a single counter inside
--     the events package (see decision_log.md D6).
--   - payload is JSONB; the kind union and field shapes are validated at the
--     application layer (Go), not in SQL.
--   - candidate_corrections is intentionally per-session — the design forbids
--     cross-session candidate memory.

-- ─────────────────────────────────────────────────────────────
-- 1. Per-attempt session metadata
-- ─────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS candidate_sessions (
    session_id      UUID            PRIMARY KEY DEFAULT gen_random_uuid(),
    candidate_id    TEXT            NOT NULL,            -- handle for now (e.g. "alice"); see decision_log D3
    challenge_id    TEXT            NOT NULL,            -- joins problems.slug
    difficulty      TEXT            NOT NULL CHECK (difficulty IN ('easy','medium','hard')),
    candidate_role  TEXT,                                -- self-reported, e.g. "senior_backend"

    started_at      TIMESTAMPTZ     NOT NULL DEFAULT now(),
    submitted_at    TIMESTAMPTZ,

    total_turns     INTEGER         NOT NULL DEFAULT 0,
    tool_use_count  INTEGER         NOT NULL DEFAULT 0,
    abort_reason    TEXT,                                -- normal | aborted | max_turns | budget

    -- 5-dim scores (grader writes; nullable per dimension because not every
    -- difficulty evaluates every dimension)
    score_correctness  NUMERIC(5,2),
    score_decompose    NUMERIC(5,2),                     -- hard only
    score_collab       NUMERIC(5,2),
    score_verify       NUMERIC(5,2),                     -- medium/hard only
    score_comm         NUMERIC(5,2),                     -- hard only
    final_score        NUMERIC(5,2),

    grader_engine      TEXT,                             -- 'gemini' | 'anthropic'
    graded_at          TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_candidate_sessions_candidate
    ON candidate_sessions(candidate_id, started_at DESC);
CREATE INDEX IF NOT EXISTS idx_candidate_sessions_challenge
    ON candidate_sessions(challenge_id, submitted_at DESC);

-- ─────────────────────────────────────────────────────────────
-- 2. Structured event stream (grader's primary input)
-- ─────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS session_events (
    session_id  UUID            NOT NULL REFERENCES candidate_sessions(session_id) ON DELETE CASCADE,
    seq         BIGINT          NOT NULL,
    kind        TEXT            NOT NULL,
    emitted_at  TIMESTAMPTZ     NOT NULL DEFAULT now(),
    payload     JSONB           NOT NULL,

    PRIMARY KEY (session_id, seq)
);

CREATE INDEX IF NOT EXISTS idx_session_events_kind
    ON session_events(session_id, kind);

-- ─────────────────────────────────────────────────────────────
-- 3. Chat transcript (audit + Communication dimension)
-- ─────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS session_messages (
    session_id  UUID            NOT NULL REFERENCES candidate_sessions(session_id) ON DELETE CASCADE,
    message_id  TEXT            NOT NULL,
    role        TEXT            NOT NULL CHECK (role IN ('user','assistant','tool')),
    content     JSONB           NOT NULL,                -- Anthropic content blocks: text / tool_use / tool_result
    seq         BIGINT          NOT NULL,                -- shares counter with session_events.seq
    created_at  TIMESTAMPTZ     NOT NULL DEFAULT now(),

    PRIMARY KEY (session_id, message_id)
);

CREATE INDEX IF NOT EXISTS idx_session_messages_seq
    ON session_messages(session_id, seq);

-- ─────────────────────────────────────────────────────────────
-- 4. Per-session candidate corrections (feedback type memory)
-- ─────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS session_candidate_corrections (
    session_id          UUID            NOT NULL REFERENCES candidate_sessions(session_id) ON DELETE CASCADE,
    seq                 BIGINT          NOT NULL,
    correction_text     TEXT            NOT NULL,
    source_event_seq    BIGINT,                          -- back-reference into session_events.seq
    emitted_at          TIMESTAMPTZ     NOT NULL DEFAULT now(),

    PRIMARY KEY (session_id, seq)
);
