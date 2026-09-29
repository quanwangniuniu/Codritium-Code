export type Difficulty = "easy" | "medium" | "hard";
export type Category = "debugging" | "refactoring" | "feature_build" | "security" | "company_premium";
export type AiTool = "claude_code" | "cursor" | "copilot" | "chatgpt" | "claude_app" | "other";

// One open editor tab in the IDE workspace. `dirty` flips when the buffer
// drifts from the last-persisted starter file content.
export type OpenTab = { name: string; dirty: boolean };

export interface User {
  id: string;
  display_name: string;
  avatar_url: string;
  github_handle: string;
  region: string;
  streak_current: number;
  streak_longest: number;
  streak_last_completed_at: string | null;
  is_pro: boolean;
  tier?: "standard" | "pro" | "max";
  credits?: number;
  role?: "user" | "admin";
  email?: string;
}

export interface Company {
  slug: string;
  name: string;
  region: string;
  ai_policy_summary: string;
  ai_policy_source_url: string;
}

export interface Problem {
  id: string;
  title: string;
  category: Category;
  difficulty: Difficulty;
  description_md: string;
  canva_fuzzification_prefix_md: string | null;
  starter_files: Record<string, string>;
  hint: string | null;
  is_pro_only: boolean;
  ray_seed: boolean;
  company_slugs: string[];
}

export interface DimensionScore {
  score: number | null;
  reasoning: string;
}

export interface ScoreBreakdown {
  correctness: DimensionScore;
  problem_decomposition: DimensionScore;
  ai_collaboration: DimensionScore;
  verification_quality: DimensionScore;
  communication: DimensionScore;
  total: number;
  weights_applied: Record<string, number>;
  judge_model: string;
}

export interface AntiPatternFlag {
  evaluated: boolean;
  triggered: boolean;
  evidence: string;
}

export interface AntiPatternBundle {
  hands_off: AntiPatternFlag;
  feature_marathon: AntiPatternFlag;
  ai_showcase: AntiPatternFlag;
  not_thinking: AntiPatternFlag;
}

export interface Submission {
  id: string;
  user_id: string;
  problem_id: string;
  status: "pending" | "grading" | "completed" | "failed";
  submitted_files: Record<string, string>;
  prompt_history: string;
  ai_tool_used: AiTool;
  ai_model_used: string;
  num_prompts: number;
  time_spent_seconds: number;
  submitted_at: string;
  graded_at: string | null;
  session_id?: string;
  score: ScoreBreakdown | null;
  anti_patterns: AntiPatternBundle | null;
  test_pass_rate: number;
  test_results: Record<string, "pass" | "fail">;
}

export interface DailyChallenge {
  challenge_date: string;
  problem_id: string;
}

export interface PublicSolution {
  id: string;
  author_id: string;
  problem_id: string;
  submission_id: string;
  title: string;
  writeup_md: string;
  approach_label: string;
  used_by_count: number;
  upvote_count: number;
  created_at: string;
}
