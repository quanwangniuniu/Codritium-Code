-- Community forum (/forums): posts in sections, up/down votes, one view per
-- signed-in user, and comments with one level of replies. Kept separate from
-- the per-problem `comments` table, which has its own graded gate.
--
-- Soft delete via deleted_at everywhere; vote/view/comment tallies are
-- denormalized counters maintained by the forum API.

CREATE TABLE IF NOT EXISTS forum_posts (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    section       TEXT NOT NULL
                  CHECK (section IN ('interview','career','compensation','feedback','problems')),
    -- Optional link to the problem a 'problems' post discusses.
    problem_slug  TEXT,
    title         TEXT NOT NULL,
    body_md       TEXT NOT NULL,
    tags          TEXT[] NOT NULL DEFAULT '{}',
    -- Anonymous posting is only offered in interview/compensation.
    is_anonymous  BOOLEAN NOT NULL DEFAULT false
                  CHECK (NOT is_anonymous OR section IN ('interview','compensation')),
    is_pinned     BOOLEAN NOT NULL DEFAULT false,
    upvotes       INT NOT NULL DEFAULT 0,
    downvotes     INT NOT NULL DEFAULT 0,
    view_count    INT NOT NULL DEFAULT 0,
    comment_count INT NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at    TIMESTAMPTZ,
    search_vector TSVECTOR GENERATED ALWAYS AS (
        setweight(to_tsvector('english', title), 'A') ||
        setweight(to_tsvector('english', body_md), 'B')
    ) STORED
);
CREATE INDEX IF NOT EXISTS idx_forum_posts_section_created
    ON forum_posts(section, created_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_forum_posts_created
    ON forum_posts(created_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_forum_posts_pinned
    ON forum_posts(created_at DESC) WHERE is_pinned AND deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_forum_posts_search
    ON forum_posts USING GIN (search_vector);
CREATE INDEX IF NOT EXISTS idx_forum_posts_tags
    ON forum_posts USING GIN (tags);
CREATE INDEX IF NOT EXISTS idx_forum_posts_user
    ON forum_posts(user_id);

CREATE TABLE IF NOT EXISTS forum_post_votes (
    post_id  UUID NOT NULL REFERENCES forum_posts(id) ON DELETE CASCADE,
    user_id  UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    value    INT NOT NULL CHECK (value IN (-1, 1)),
    voted_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (post_id, user_id)
);

-- One row per (post, signed-in viewer); view_count only increments on the
-- first insert, so reloading a post doesn't inflate it.
CREATE TABLE IF NOT EXISTS forum_post_views (
    post_id   UUID NOT NULL REFERENCES forum_posts(id) ON DELETE CASCADE,
    user_id   UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    viewed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (post_id, user_id)
);

CREATE TABLE IF NOT EXISTS forum_comments (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    post_id      UUID NOT NULL REFERENCES forum_posts(id) ON DELETE CASCADE,
    user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    -- Replies are one level deep: parent_id always points at a top-level
    -- comment of the same post (enforced by the API).
    parent_id    UUID REFERENCES forum_comments(id) ON DELETE CASCADE,
    body         TEXT NOT NULL,
    is_anonymous BOOLEAN NOT NULL DEFAULT false,
    upvotes      INT NOT NULL DEFAULT 0,
    downvotes    INT NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at   TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_forum_comments_post
    ON forum_comments(post_id, created_at) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_forum_comments_parent
    ON forum_comments(parent_id) WHERE parent_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_forum_comments_user
    ON forum_comments(user_id);

CREATE TABLE IF NOT EXISTS forum_comment_votes (
    comment_id UUID NOT NULL REFERENCES forum_comments(id) ON DELETE CASCADE,
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    value      INT NOT NULL CHECK (value IN (-1, 1)),
    voted_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (comment_id, user_id)
);
