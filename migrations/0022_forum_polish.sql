-- Forum phase 5: images in posts and comments, and post edit history.

-- Images uploaded into posts and comments, served at
-- /api/forum/images/{id}. Like avatars they live in Postgres: they're
-- resized in the browser and capped server-side, so they stay small.
CREATE TABLE IF NOT EXISTS forum_images (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    content_type TEXT NOT NULL,
    data         BYTEA NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_forum_images_user_created
    ON forum_images(user_id, created_at DESC);

-- A post's earlier versions: each edit saves what it replaced.
CREATE TABLE IF NOT EXISTS forum_post_revisions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    post_id     UUID NOT NULL REFERENCES forum_posts(id) ON DELETE CASCADE,
    title       TEXT NOT NULL,
    body_md     TEXT NOT NULL,
    tags        TEXT[] NOT NULL DEFAULT '{}',
    -- When this version was current from (the post's created_at or the
    -- previous edit) and when it was replaced.
    written_at  TIMESTAMPTZ NOT NULL,
    replaced_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_forum_post_revisions_post
    ON forum_post_revisions(post_id, replaced_at DESC);
