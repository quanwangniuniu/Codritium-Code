-- Add soul_prebake column to problems.
-- soul_prebake holds the per-problem authored guidance (markdown) that the
-- tips-agent loads as system_instruction. Source of truth is the arena
-- research phase-C 05_soul_md_prebake.md file; the import script copies
-- the content into this column. Empty string for problems without prebake.

ALTER TABLE problems
    ADD COLUMN IF NOT EXISTS soul_prebake TEXT NOT NULL DEFAULT '';
