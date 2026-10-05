-- Forum v2 (phase 1):
--   * Sections follow LeetCode Discuss: Interview Question, Interview
--     Experience ('interview'), Compensation, Career, Study Guide, General
--     Discussion, Feedback. Per-problem discussion lives on each problem's
--     page, so the old 'problems' section folds into 'general'; a post can
--     still link a problem from any section.
--   * Signed-out readers count as views too, once per visitor cookie per day.

-- Drop the section and anonymity checks by lookup: both were created inline,
-- so their names were picked by Postgres.
DO $$
DECLARE r record;
BEGIN
  FOR r IN
    SELECT conname FROM pg_constraint
    WHERE conrelid = 'forum_posts'::regclass AND contype = 'c'
      AND pg_get_constraintdef(oid) LIKE '%section%'
  LOOP
    EXECUTE format('ALTER TABLE forum_posts DROP CONSTRAINT %I', r.conname);
  END LOOP;
END $$;

UPDATE forum_posts SET section = 'general' WHERE section = 'problems';

ALTER TABLE forum_posts
  ADD CONSTRAINT forum_posts_section_check CHECK (section IN (
    'interview-question','interview','compensation','career',
    'study-guide','general','feedback')),
  ADD CONSTRAINT forum_posts_anonymous_check CHECK (
    NOT is_anonymous OR section IN ('interview-question','interview','compensation'));

-- One row per (post, signed-out visitor cookie). viewed_at is the last time
-- the view was counted; a visit counts again only once it is a day old.
CREATE TABLE IF NOT EXISTS forum_post_visitor_views (
    post_id    UUID NOT NULL REFERENCES forum_posts(id) ON DELETE CASCADE,
    visitor_id UUID NOT NULL,
    viewed_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (post_id, visitor_id)
);

-- Rate limits count a user's recent posts, comments, and votes.
CREATE INDEX IF NOT EXISTS idx_forum_posts_user_created
    ON forum_posts(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_forum_comments_user_created
    ON forum_comments(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_forum_post_votes_user_voted
    ON forum_post_votes(user_id, voted_at DESC);
CREATE INDEX IF NOT EXISTS idx_forum_comment_votes_user_voted
    ON forum_comment_votes(user_id, voted_at DESC);
