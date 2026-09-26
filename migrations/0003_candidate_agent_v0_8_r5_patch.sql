-- v0.8 candidate-agent migration · R5 patch
--
-- Adds the columns required by the R5 Gonfire research absorption (HANDOFF §11
-- + events_payload_spec.md §6.1):
--   1. five phase_<X>_percent columns on candidate_sessions, populated by a
--      post-grading LLM phase-label call. These are diagnostic only — they
--      MUST NOT enter final_score (see HANDOFF §11.1).
--   2. one summary JSONB column on candidate_sessions, populated by the
--      grader at scoring time, holding aggregate metrics derived from the
--      session_events stream (tool_use_counts / thinking_time / revert_count
--      / tool_error_count).
--
-- "Verification" appears both as a 5-dim scoring dimension and as a phase
-- name. They are NOT the same concept — the phase is a time-bucket label
-- for diagnostic reporting; the dimension is a weighted score. Reasoning
-- prompts in the grader must keep them separated.

ALTER TABLE candidate_sessions
    ADD COLUMN IF NOT EXISTS phase_orientation_percent     NUMERIC(5,2),
    ADD COLUMN IF NOT EXISTS phase_planning_percent        NUMERIC(5,2),
    ADD COLUMN IF NOT EXISTS phase_implementation_percent  NUMERIC(5,2),
    ADD COLUMN IF NOT EXISTS phase_verification_percent    NUMERIC(5,2),
    ADD COLUMN IF NOT EXISTS phase_diagnosis_percent       NUMERIC(5,2),
    ADD COLUMN IF NOT EXISTS summary                       JSONB;

-- summary JSONB shape (validated at the application layer, not in SQL):
--   {
--     "tool_use_counts":         { "reads": N, "writes": N, "bash": N, "other": N },
--     "thinking_time_median_sec": N,
--     "thinking_time_longest_sec": N,
--     "revert_count":             N,
--     "tool_error_count":         N
--   }
