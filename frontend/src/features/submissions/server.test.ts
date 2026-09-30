import { beforeEach, describe, expect, it, vi } from "vitest";

const apiJSON = vi.fn();
vi.mock("@/shared/api/server", () => ({ apiJSON: (path: string) => apiJSON(path) }));

import { listProblems } from "@/features/problems/server";
import { getSubmission, listUserProblemSubmissions } from "@/features/submissions/server";

beforeEach(() => {
  apiJSON.mockReset();
});

describe("listProblems", () => {
  it("adapts the backend brief shape", async () => {
    apiJSON.mockResolvedValueOnce([
      { slug: "fix-it", title: "Fix it", category: "debugging", difficulty: "easy", requires_pro: true },
    ]);
    const [p] = await listProblems();
    expect(p).toMatchObject({ id: "fix-it", title: "Fix it", category: "debugging", difficulty: "easy", is_pro_only: true });
  });

  it("returns [] when the backend answers null", async () => {
    apiJSON.mockResolvedValueOnce(null);
    expect(await listProblems()).toEqual([]);
  });
});

describe("getSubmission", () => {
  it("maps status, pass rate, scores and anti-patterns", async () => {
    apiJSON.mockResolvedValueOnce({
      id: "s1",
      user_id: "u1",
      problem_slug: "fix-it",
      status: "graded",
      variant: "as-is",
      code_files: { "a.py": "x" },
      test_results: { tests: [{ passed: true }, { passed: false }] },
      scores: {
        final_score: 72,
        dimension_scores: {
          correctness: { score: 4, reasoning: "ok" },
          verification: { score: 3, reasoning: "some tests" },
        },
        weights: { correctness: 0.3, verification: 0.2 },
        anti_patterns: { hands_off: { evaluated: true, triggered: true, evidence: "e" } },
        judge_model: "judge",
      },
      final_score: 72,
      submitted_at: "2026-01-01T00:00:00Z",
      graded_at: null,
      session_id: "sess",
    });
    const s = await getSubmission("s1");
    expect(apiJSON).toHaveBeenCalledWith("/api/submissions/s1");
    expect(s?.status).toBe("completed");
    expect(s?.test_pass_rate).toBe(0.5);
    expect(s?.session_id).toBe("sess");
    expect(s?.score?.total).toBe(72);
    expect(s?.score?.verification_quality).toEqual({ score: 3, reasoning: "some tests" });
    expect(s?.score?.problem_decomposition).toEqual({ score: null, reasoning: "" });
    expect(s?.score?.weights_applied).toEqual({ correctness: 0.3, verification_quality: 0.2 });
    expect(s?.anti_patterns?.hands_off).toEqual({ evaluated: true, triggered: true, evidence: "e" });
    expect(s?.anti_patterns?.not_thinking).toEqual({ evaluated: false, triggered: false, evidence: "" });
  });

  it("returns null scores for an ungraded submission and unknown status -> pending", async () => {
    apiJSON.mockResolvedValueOnce({
      id: "s2",
      user_id: "u1",
      problem_slug: "p",
      status: "weird",
      variant: "as-is",
      code_files: null,
      test_results: null,
      scores: null,
      final_score: null,
      submitted_at: "",
      graded_at: null,
    });
    const s = await getSubmission("s2");
    expect(s?.status).toBe("pending");
    expect(s?.score).toBeNull();
    expect(s?.anti_patterns).toBeNull();
    expect(s?.submitted_files).toEqual({});
  });
});

describe("listUserProblemSubmissions", () => {
  it("filters the list by problem and keeps final_score in score.total", async () => {
    apiJSON.mockResolvedValueOnce([
      { id: "a", problem_slug: "p1", status: "graded", final_score: 80, submitted_at: "", graded_at: null },
      { id: "b", problem_slug: "p2", status: "grading", final_score: null, submitted_at: "", graded_at: null },
    ]);
    const out = await listUserProblemSubmissions("", "p1");
    expect(out.map((s) => s.id)).toEqual(["a"]);
    expect(out[0].status).toBe("completed");
    expect(out[0].score?.total).toBe(80);
  });
});
