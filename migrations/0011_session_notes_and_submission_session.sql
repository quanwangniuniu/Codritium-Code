-- Codritium 1.0: candidate notes attached to a session, and a foreign key
-- from submissions back to the candidate session so the grader can pull the
-- chat_v2 conversation from session_messages instead of the legacy
-- submission_events stream.

-- session_notes: free-form markdown a candidate writes while replaying their
-- run. shared_to_comment_id is set when the candidate shares the note into
-- the per-problem discussion; null until then.
CREATE TABLE IF NOT EXISTS session_notes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id UUID NOT NULL REFERENCES candidate_sessions(session_id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    body TEXT NOT NULL,
    shared_to_comment_id UUID REFERENCES comments(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_notes_session ON session_notes(session_id);
CREATE INDEX IF NOT EXISTS idx_notes_user ON session_notes(user_id);

-- submissions.session_id ties a submission to the candidate session that
-- produced its chat_v2 transcript. ON DELETE SET NULL keeps historical
-- submissions visible when an upstream session is purged.
ALTER TABLE submissions
    ADD COLUMN IF NOT EXISTS session_id UUID REFERENCES candidate_sessions(session_id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_submissions_session ON submissions(session_id);
