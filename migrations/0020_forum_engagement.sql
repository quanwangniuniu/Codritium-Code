-- Forum phase 2: in-app notifications, bookmarks, and post follows.

-- One row per thing a user should hear about. actor_id is who caused it
-- (NULL for system events like vote milestones); actor_anonymous hides the
-- actor from the recipient when the triggering comment was anonymous.
CREATE TABLE IF NOT EXISTS notifications (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind            TEXT NOT NULL CHECK (kind IN (
                      'post_reply',      -- someone commented on your post
                      'comment_reply',   -- someone replied to your comment
                      'mention',         -- someone @mentioned you
                      'post_comment',    -- new comment on a post you follow
                      'post_milestone'   -- your post reached N upvotes
                    )),
    actor_id        UUID REFERENCES users(id) ON DELETE CASCADE,
    actor_anonymous BOOLEAN NOT NULL DEFAULT false,
    post_id         UUID NOT NULL REFERENCES forum_posts(id) ON DELETE CASCADE,
    comment_id      UUID REFERENCES forum_comments(id) ON DELETE CASCADE,
    milestone       INT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    read_at         TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_notifications_user_created
    ON notifications(user_id, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_notifications_user_unread
    ON notifications(user_id) WHERE read_at IS NULL;
-- A post announces each milestone once, even if votes go down and back up.
CREATE UNIQUE INDEX IF NOT EXISTS idx_notifications_milestone_once
    ON notifications(user_id, post_id, milestone) WHERE kind = 'post_milestone';

CREATE TABLE IF NOT EXISTS forum_bookmarks (
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    post_id    UUID NOT NULL REFERENCES forum_posts(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, post_id)
);
CREATE INDEX IF NOT EXISTS idx_forum_bookmarks_user_created
    ON forum_bookmarks(user_id, created_at DESC);

-- Followers hear about every new comment on the post.
CREATE TABLE IF NOT EXISTS forum_post_follows (
    post_id    UUID NOT NULL REFERENCES forum_posts(id) ON DELETE CASCADE,
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (post_id, user_id)
);

-- Authors follow their own posts, including ones written before follows existed.
INSERT INTO forum_post_follows (post_id, user_id)
SELECT id, user_id FROM forum_posts WHERE deleted_at IS NULL
ON CONFLICT DO NOTHING;

-- Mention lookup and autocomplete match handles case-insensitively.
CREATE INDEX IF NOT EXISTS idx_users_handle_lower ON users (lower(handle) text_pattern_ops);
