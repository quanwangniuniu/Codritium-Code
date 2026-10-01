// Server-side submission reads. Adapts the backend submission / scores
// blobs into the UI's Submission type.
import { apiJSON } from "@/shared/api/server";
import type { Submission } from "./types";

// Backend submission status -> UI status. Shared by the detail and list
// adapters; anything unknown renders as pending.
const SUBMISSION_STATUS: Record<string, Submission["status"]> = {
  pending: "pending",
  grading: "grading",
  graded: "completed",
  completed: "completed",
  failed: "failed",
};

function adaptStatus(status: string): Submission["status"] {
  return SUBMISSION_STATUS[status] ?? "pending";
}

interface BackendSubmission {
  id: string;
  user_id: string;
  problem_slug: string;
  status: string;
  variant: string;
  code_files: Record<string, string>;
  test_results: unknown;
  scores: unknown;
  final_score: number | null;
  submitted_at: string;
  graded_at: string | null;
  session_id?: string;
}

interface BackendAntiPatternFlag {
  evaluated?: boolean;
  triggered?: boolean;
  evidence?: string;
}

interface BackendScores {
  dimension_scores?: Record<string, { score?: number | null; reasoning?: string }>;
  final_score?: number;
  weights?: Record<string, number>;
  anti_patterns?: {
    hands_off?: BackendAntiPatternFlag;
    feature_marathon?: BackendAntiPatternFlag;
    ai_showcase?: BackendAntiPatternFlag;
    not_thinking?: BackendAntiPatternFlag;
  };
  judge_model?: string;
}

function adaptScores(raw: unknown): Submission["score"] {
  if (!raw || typeof raw !== "object") return null;

  const s = raw as BackendScores;
  if (!s.dimension_scores || typeof s.final_score !== "number") {
    return null;
  }

  const ds = s.dimension_scores;
  // Backend stores the verification dim under "verification"; demo2 UI uses
  // "verification_quality". Translate both keys in adapter.
  const dim = (key: string, fallback?: string) => ({
    score: ds[key]?.score ?? (fallback ? ds[fallback]?.score : null) ?? null,
    reasoning: ds[key]?.reasoning ?? (fallback ? ds[fallback]?.reasoning : "") ?? "",
  });
  // Normalize weights: backend "verification" → UI "verification_quality".
  const backendWeights = s.weights ?? {};
  const weights: Record<string, number> = {};
  for (const [k, v] of Object.entries(backendWeights)) {
    const out = k === "verification" ? "verification_quality" : k;
    weights[out] = v;
  }
  return {
    correctness: dim("correctness"),
    problem_decomposition: dim("problem_decomposition"),
    ai_collaboration: dim("ai_collaboration"),
    verification_quality: dim("verification_quality", "verification"),
    communication: dim("communication"),
    total: s.final_score ?? 0,
    weights_applied: weights,
    judge_model: s.judge_model ?? "",
  };
}

function adaptAntiPatterns(raw: unknown): Submission["anti_patterns"] {
  if (!raw || typeof raw !== "object") return null;

  const source = (raw as BackendScores).anti_patterns;
  const flag = (value?: BackendAntiPatternFlag) => ({
    evaluated: value?.evaluated ?? false,
    triggered: value?.triggered ?? false,
    evidence: value?.evidence ?? "",
  });

  return {
    hands_off: flag(source?.hands_off),
    feature_marathon: flag(source?.feature_marathon),
    ai_showcase: flag(source?.ai_showcase),
    not_thinking: flag(source?.not_thinking),
  };
}

function adaptSubmission(s: BackendSubmission): Submission {
  // Pull a simple pass-rate from backend test_results shape: {tests: [{name, passed}, ...]}
  let passRate = 0;
  const tr = s.test_results as { tests?: { passed?: boolean }[] } | null;
  if (tr && Array.isArray(tr.tests) && tr.tests.length > 0) {
    const passed = tr.tests.filter((t) => t.passed).length;
    passRate = passed / tr.tests.length;
  }
  return {
    id: s.id,
    user_id: s.user_id,
    problem_id: s.problem_slug,
    status: adaptStatus(s.status),
    submitted_files: s.code_files ?? {},
    prompt_history: "",
    ai_tool_used: "other",
    ai_model_used: "",
    num_prompts: 0,
    time_spent_seconds: 0,
    submitted_at: s.submitted_at,
    graded_at: s.graded_at,
    score: adaptScores(s.scores),
    anti_patterns: adaptAntiPatterns(s.scores),
    test_pass_rate: passRate,
    test_results: {},
    session_id: s.session_id,
  };
}

export async function getSubmission(id: string): Promise<Submission | null> {
  const s = await apiJSON<BackendSubmission>(
    `/api/submissions/${encodeURIComponent(id)}`,
  );
  return s ? adaptSubmission(s) : null;
}

interface BackendSubmissionListItem {
  id: string;
  problem_slug: string;
  problem_title: string;
  category: string;
  difficulty: string;
  status: string;
  variant: string;
  final_score: number | null;
  submitted_at: string;
  graded_at: string | null;
}

function adaptListItem(s: BackendSubmissionListItem): Submission {
  // Profile / submission lists only need `score?.total` and status — so the
  // minimum ScoreBreakdown shape carries the final_score in `total`. Detail
  // page goes through getSubmission, which has the full scores blob.
  const score = s.final_score != null
    ? {
      correctness: { score: null, reasoning: "" },
      problem_decomposition: { score: null, reasoning: "" },
      ai_collaboration: { score: null, reasoning: "" },
      verification_quality: { score: null, reasoning: "" },
      communication: { score: null, reasoning: "" },
      total: s.final_score,
      weights_applied: {},
      judge_model: "",
    }
    : null;
  return {
    id: s.id,
    user_id: "",
    problem_id: s.problem_slug,
    status: adaptStatus(s.status),
    submitted_files: {},
    prompt_history: "",
    ai_tool_used: "other",
    ai_model_used: "",
    num_prompts: 0,
    time_spent_seconds: 0,
    submitted_at: s.submitted_at,
    graded_at: s.graded_at,
    score,
    anti_patterns: null,
    test_pass_rate: 0,
    test_results: {},
  };
}

export async function listUserSubmissions(_userId: string): Promise<Submission[]> {
  const out = (await apiJSON<BackendSubmissionListItem[]>("/api/me/submissions")) ?? [];
  return out.map(adaptListItem);
}

export async function listUserProblemSubmissions(
  _userId: string,
  problemId: string,
): Promise<Submission[]> {
  // Backend has no per-problem filter; client-side filter on the full list.
  const all = await listUserSubmissions("");
  return all.filter((s) => s.problem_id === problemId);
}
