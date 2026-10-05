-- Forum phase 4: reports, a moderation queue, thread locks, and mutes.

-- A reader flags a post or a comment (comment_id set). Reports stay 'open'
-- until an admin resolves the target: 'actioned' (content removed) or
-- 'dismissed' (nothing wrong).
CREATE TABLE IF NOT EXISTS forum_reports (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    reporter_id  UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    post_id      UUID NOT NULL REFERENCES forum_posts(id) ON DELETE CASCADE,
    comment_id   UUID REFERENCES forum_comments(id) ON DELETE CASCADE,
    reason       TEXT NOT NULL CHECK (reason IN ('spam','abuse','off_topic','other')),
    details      TEXT NOT NULL DEFAULT '',
    status       TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open','actioned','dismissed')),
    resolved_by  UUID REFERENCES users(id) ON DELETE SET NULL,
    resolved_at  TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- One open report per reader per target.
CREATE UNIQUE INDEX IF NOT EXISTS idx_forum_reports_open_once
    ON forum_reports(reporter_id, post_id, COALESCE(comment_id, '00000000-0000-0000-0000-000000000000'))
    WHERE status = 'open';
CREATE INDEX IF NOT EXISTS idx_forum_reports_open
    ON forum_reports(created_at) WHERE status = 'open';
CREATE INDEX IF NOT EXISTS idx_forum_reports_reporter_created
    ON forum_reports(reporter_id, created_at DESC);

-- A locked thread takes no new comments or edits; votes and reading go on.
ALTER TABLE forum_posts ADD COLUMN IF NOT EXISTS is_locked BOOLEAN NOT NULL DEFAULT false;

-- A muted user can read but not post, comment, edit, vote, or report until
-- muted_until.
CREATE TABLE IF NOT EXISTS forum_mutes (
    user_id     UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    muted_until TIMESTAMPTZ NOT NULL,
    reason      TEXT NOT NULL DEFAULT '',
    muted_by    UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Reputation sums upvotes on a user's named posts and comments.
CREATE INDEX IF NOT EXISTS idx_forum_comments_user_live
    ON forum_comments(user_id) WHERE deleted_at IS NULL AND NOT is_anonymous;
CREATE INDEX IF NOT EXISTS idx_forum_posts_user_live
    ON forum_posts(user_id) WHERE deleted_at IS NULL AND NOT is_anonymous;
