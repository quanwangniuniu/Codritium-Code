// Frontend-v2 data layer for Codritium backend (:8080). Demo2 originally
// fronted a richer mock store; the Codritium backend only exposes a subset of
// those endpoints. This file:
//   - Forwards problems/submissions/me to real Codritium endpoints.
//   - Adapts Codritium response shapes into the demo2 UI shapes already
//     consumed by pages (so the pages compile unchanged).
//   - Stubs endpoints the backend does not implement yet (companies,
//     daily-challenge, solutions, by-handle user lookup,
//     submissions history) with empty results. Wiring those is I6 territory.
import { apiJSON } from "@/lib/api-server";
import type {
  Company,
  DailyChallenge,
  Problem,
  PublicSolution,
  Submission,
  User,
  ProfileData,
} from "@/lib/types";

// --- Problem adapter ----------------------------------------------------

interface BackendProblemBrief {
  slug: string;
  title: string;
  category: string;
  difficulty: string;
  requires_pro?: boolean;
}

interface BackendProblemFull extends BackendProblemBrief {
  readme_md: string;
  starter_files: Record<string, string>;
  variant: string;
  strip_variant: string;
}

function adaptProblemBrief(p: BackendProblemBrief): Problem {
  return {
    id: p.slug,
    title: p.title,
    category: p.category as Problem["category"],
    difficulty: p.difficulty as Problem["difficulty"],
    description_md: "",
    canva_fuzzification_prefix_md: null,
    starter_files: {},
    hint: null,
    is_pro_only: p.requires_pro ?? false,
    ray_seed: false,
    company_slugs: [],
  };
}

function adaptProblemFull(p: BackendProblemFull): Problem {
  return {
    id: p.slug,
    title: p.title,
    category: p.category as Problem["category"],
    difficulty: p.difficulty as Problem["difficulty"],
    description_md: p.readme_md,
    canva_fuzzification_prefix_md: null,
    starter_files: p.starter_files ?? {},
    hint: null,
    is_pro_only: p.requires_pro ?? false,
    ray_seed: false,
    company_slugs: [],
  };
}

// --- Submission adapter -------------------------------------------------

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
  const statusMap: Record<string, Submission["status"]> = {
    pending: "pending",
    grading: "grading",
    graded: "completed",
    completed: "completed",
    failed: "failed",
  };
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
    status: statusMap[s.status] ?? "pending",
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

// --- Problems ---------------------------------------------------------

export async function listProblems(): Promise<Problem[]> {
  const out = (await apiJSON<BackendProblemBrief[]>("/api/problems")) ?? [];
  return out.map(adaptProblemBrief);
}

export interface AttemptedProblem {
  slug: string;
  last_started_at: string;
}

interface BackendAttemptedProblem {
  challenge_slug: string;
  last_started_at: string;
}

export async function listMyAttemptedProblems(): Promise<AttemptedProblem[]> {
  const out = (await apiJSON<BackendAttemptedProblem[]>("/api/me/attempted")) ?? [];
  return out.map((p) => ({
    slug: p.challenge_slug,
    last_started_at: p.last_started_at,
  }));
}

export async function getProblem(id: string): Promise<Problem | null> {
  const p = await apiJSON<BackendProblemFull>(
    `/api/problems/${encodeURIComponent(id)}`,
  );
  return p ? adaptProblemFull(p) : null;
}

// --- Submissions ------------------------------------------------------

export async function getSubmission(id: string): Promise<Submission | null> {
  const s = await apiJSON<BackendSubmission>(
    `/api/submissions/${encodeURIComponent(id)}`,
  );
  return s ? adaptSubmission(s) : null;
}

// Reply payload shape mirrors what /api/challenges/{slug}/official-reply
// returns. envelopes is kept as the raw ReplyEnvelope[] so the client
// component can run transformForReplay on it. files / starter_files /
// explanation_md feed the two-column replay UI: file tree, cumulative
// diff baseline, and standard-solution prose respectively.
export interface OfficialReplyResponse {
  source: string;
  challenge_slug: string;
  generator_model?: string;
  candidate_index?: number;
  created_at?: string;
  envelopes: unknown[];
  files?: Record<string, string>;
  starter_files?: Record<string, string>;
  explanation_md?: string;
}

export async function getOfficialReply(
  slug: string,
): Promise<OfficialReplyResponse | null> {
  return apiJSON<OfficialReplyResponse>(
    `/api/challenges/${encodeURIComponent(slug)}/official-reply`,
  );
}

// MyReplayResponse reuses the same envelope shape as the official reply
// but is fed from session_events for the candidate's most recent run on
// the slug. Returns null when the candidate has not started the problem
// yet (backend responds 404, the apiJSON helper translates to null).
export interface MyReplayResponse extends OfficialReplyResponse {
  session_id?: string;
  started_at?: string;
}

export async function getMyReplay(
  slug: string,
): Promise<MyReplayResponse | null> {
  return apiJSON<MyReplayResponse>(
    `/api/me/replays/${encodeURIComponent(slug)}`,
  );
}

// Per-session replay for a specific submission's run, keyed by the
// submission's session_id (so a candidate with several attempts sees the
// run that belongs to the submission they opened, not just the latest).
// Returns null when the session has no recorded events.
export async function getSessionReply(
  sessionId: string,
): Promise<MyReplayResponse | null> {
  return apiJSON<MyReplayResponse>(
    `/api/sessions/${encodeURIComponent(sessionId)}/reply`,
  );
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
  const statusMap: Record<string, Submission["status"]> = {
    pending: "pending",
    grading: "grading",
    graded: "completed",
    failed: "failed",
  };
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
    status: statusMap[s.status] ?? "pending",
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

// --- Profile ----------------------------------------------------------

export async function getMyProfile(): Promise<ProfileData | null> {
  return apiJSON<ProfileData>("/api/me/profile");
}

// --- Stubs: backend does not implement these endpoints yet ------------

export async function listCompanies(): Promise<Company[]> {
  return [];
}

export async function getDailyChallenge(): Promise<DailyChallenge | null> {
  return null;
}

export async function listAllUsers(): Promise<User[]> {
  return [];
}

export async function getUserByHandle(_handle: string): Promise<User | null> {
  return null;
}

export async function listProblemSolutions(_problemId: string): Promise<PublicSolution[]> {
  return [];
}
