-- Extend solutions_replies with the artifacts the two-column replay UI
-- needs: a final file map for the "Answer mode" diff baseline, a written
-- explanation rendered as standard markdown, and a snapshot of the
-- starter files so the cumulative diff in the timeline is self-contained
-- (the table no longer has to join problems.starter_files at read time).
--
-- All three columns are nullable so the existing row stays valid until
-- T2.4.2 backfills it with the regenerated payload.

ALTER TABLE solutions_replies
    ADD COLUMN IF NOT EXISTS files          JSONB,
    ADD COLUMN IF NOT EXISTS explanation_md TEXT,
    ADD COLUMN IF NOT EXISTS starter_files  JSONB;
