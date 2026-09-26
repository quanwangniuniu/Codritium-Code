-- Codritium MVP initial schema

CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- users (demo cookie auth, 5 mock users)
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    handle TEXT UNIQUE NOT NULL,
    display_name TEXT NOT NULL,
    region TEXT,
    is_pro BOOLEAN DEFAULT FALSE,
    avatar_color TEXT DEFAULT '#7dd3fc',
    created_at TIMESTAMPTZ DEFAULT now()
);

-- problems (one mid problem preloaded; strip_variant per R10 G16)
CREATE TABLE problems (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug TEXT UNIQUE NOT NULL,
    title TEXT NOT NULL,
    category TEXT NOT NULL CHECK (category IN ('debugging','refactoring','feature_build','security')),
    difficulty TEXT NOT NULL CHECK (difficulty IN ('easy','medium','hard')),
    readme_md TEXT NOT NULL,
    starter_files JSONB NOT NULL DEFAULT '{}'::jsonb,
    strip_variant TEXT NOT NULL DEFAULT 'both' CHECK (strip_variant IN ('as-is-only','both','stripped-only')),
    hidden_test_file TEXT NOT NULL,
    created_at TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_problems_difficulty ON problems(difficulty);
CREATE INDEX idx_problems_category ON problems(category);

-- submissions (one row per submit; final_score nullable until grader finishes)
CREATE TABLE submissions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    problem_id UUID NOT NULL REFERENCES problems(id) ON DELETE CASCADE,
    variant TEXT NOT NULL DEFAULT 'as-is' CHECK (variant IN ('as-is','stripped')),
    code_files JSONB NOT NULL DEFAULT '{}'::jsonb,
    test_results JSONB,
    scores JSONB,
    final_score NUMERIC(5,2),
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','grading','graded','failed')),
    submitted_at TIMESTAMPTZ DEFAULT now(),
    graded_at TIMESTAMPTZ
);

CREATE INDEX idx_submissions_user ON submissions(user_id);
CREATE INDEX idx_submissions_problem ON submissions(problem_id);

-- submission_events (per R10 D_panel §4: candidate AI panel event stream)
CREATE TABLE submission_events (
    id BIGSERIAL PRIMARY KEY,
    submission_id UUID NOT NULL REFERENCES submissions(id) ON DELETE CASCADE,
    ts TIMESTAMPTZ DEFAULT now(),
    event_type TEXT NOT NULL CHECK (event_type IN (
        'chat_prompt','chat_response',
        'edit_request','edit_response',
        'apply_request','apply_response',
        'accept','reject',
        'user_typing_batch'
    )),
    role TEXT,
    content TEXT,
    selected_code TEXT,
    prefix TEXT,
    suffix TEXT,
    language TEXT,
    metadata JSONB DEFAULT '{}'::jsonb
);

CREATE INDEX idx_submission_events_sid_ts ON submission_events(submission_id, ts);
CREATE INDEX idx_submission_events_type ON submission_events(event_type);
