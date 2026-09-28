-- session_candidate_corrections (0002) was reserved for per-session candidate
-- feedback memory, but no code ever read or wrote it. Drop it; recreate from
-- 0002 if the feature is picked up again.

DROP TABLE IF EXISTS session_candidate_corrections;
