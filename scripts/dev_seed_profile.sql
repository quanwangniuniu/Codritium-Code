-- DEV ONLY: fills one account with a year of sample practice activity so the
-- profile page has something to render. Never run against production.
--
--   psql -h localhost -U codritium -d codritium \
--     -v email=you@example.com -f scripts/dev_seed_profile.sql
--
-- Re-running adds more rows. To undo, delete the account's submissions,
-- forum_posts, and comments.

\set ON_ERROR_STOP on

BEGIN;

CREATE TEMP TABLE seed_user ON COMMIT DROP AS
SELECT id, handle FROM users WHERE email = :'email';

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM seed_user) THEN
    RAISE EXCEPTION 'no user with that email';
  END IF;
END $$;

-- ~90 submissions spread over the past year, clustered into streaks, each
-- with five rubric scores (1-5) and a final score on the grader's 0-100 scale.
WITH days AS (
  SELECT d::date AS day
  FROM generate_series(current_date - 364, current_date, interval '1 day') AS d
  WHERE random() < CASE WHEN d > current_date - 21 THEN 0.8 ELSE 0.22 END
),
picks AS (
  SELECT day,
         (SELECT id FROM problems WHERE status = 'published'
          ORDER BY md5(day::text || slug) LIMIT 1) AS problem_id,
         1 + floor(random() * 5)::int AS c,
         1 + floor(random() * 5)::int AS pd,
         2 + floor(random() * 4)::int AS ai,
         1 + floor(random() * 4)::int AS v,
         2 + floor(random() * 4)::int AS cm
  FROM days
)
INSERT INTO submissions (user_id, problem_id, status, final_score, scores, submitted_at, graded_at)
SELECT u.id, p.problem_id, 'graded',
       round(((p.c + p.pd + p.ai + p.v + p.cm) / 5.0 * 20)::numeric, 2),
       jsonb_build_object('dimension_scores', jsonb_build_object(
         'correctness', jsonb_build_object('dimension', 'correctness', 'score', p.c),
         'problem_decomposition', jsonb_build_object('dimension', 'problem_decomposition', 'score', p.pd),
         'ai_collaboration', jsonb_build_object('dimension', 'ai_collaboration', 'score', p.ai),
         'verification', jsonb_build_object('dimension', 'verification', 'score', p.v),
         'communication', jsonb_build_object('dimension', 'communication', 'score', p.cm))),
       LEAST(now() - interval '5 minutes', p.day + time '08:00' + random() * interval '10 hours'),
       LEAST(now(), p.day + time '20:30')
FROM picks p CROSS JOIN seed_user u;

INSERT INTO forum_posts (user_id, section, title, body_md, tags, upvotes, comment_count, created_at)
SELECT u.id, v.section, v.title, v.body, v.tags, v.upvotes, v.comments, now() - v.age
FROM seed_user u,
(VALUES
  ('career', 'How I prepare for AI-enabled interviews', 'Notes from a month of practice.', ARRAY['prep'], 14, 3, interval '40 days'),
  ('problems', 'Idempotency keys vs event ids for webhook retries', 'Which one should the ledger dedupe on?', ARRAY['debugging'], 6, 2, interval '9 days'),
  ('feedback', 'Feature request: show dimension trends over time', 'Would love a chart per dimension.', ARRAY['feature'], 3, 1, interval '2 days')
) AS v(section, title, body, tags, upvotes, comments, age);

INSERT INTO comments (problem_slug, user_id, body, upvotes, created_at)
SELECT pr.slug, u.id, 'Writing the failing test first made the fix obvious.', 2, now() - interval '3 days'
FROM seed_user u, (SELECT slug FROM problems WHERE status = 'published' ORDER BY slug LIMIT 1) pr;

COMMIT;

SELECT count(*) AS submissions, count(DISTINCT problem_id) AS problems,
       count(*) FILTER (WHERE final_score >= 60) AS passing
FROM submissions WHERE user_id = (SELECT id FROM users WHERE email = :'email');
