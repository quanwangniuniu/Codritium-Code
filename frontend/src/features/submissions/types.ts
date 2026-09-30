export type AiTool = "claude_code" | "cursor" | "copilot" | "chatgpt" | "claude_app" | "other";

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
