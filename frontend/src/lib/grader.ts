import type {
  AntiPatternBundle,
  AntiPatternFlag,
  Difficulty,
  Problem,
  ScoreBreakdown,
  Submission,
} from "@/lib/types";

const WEIGHTS: Record<Difficulty, Record<string, number>> = {
  easy: { correctness: 0.7, problem_decomposition: 0, ai_collaboration: 0.3, verification_quality: 0, communication: 0 },
  medium: { correctness: 0.5, problem_decomposition: 0, ai_collaboration: 0.3, verification_quality: 0.2, communication: 0 },
  hard: { correctness: 0.25, problem_decomposition: 0.25, ai_collaboration: 0.25, verification_quality: 0.15, communication: 0.1 },
};

const ANTI_PATTERN_PENALTY: Record<keyof AntiPatternBundle, number> = {
  hands_off: 2.0,
  feature_marathon: 1.0,
  ai_showcase: 1.0,
  not_thinking: 1.5,
};

function clamp(n: number, lo: number, hi: number): number {
  return Math.max(lo, Math.min(hi, n));
}

function round2(n: number): number {
  return Math.round(n * 100) / 100;
}

function gradeCorrectness(submission: Submission, problem: Problem): { score: number; reasoning: string; testResults: Record<string, "pass" | "fail">; passRate: number } {
  const code = Object.values(submission.submitted_files).join("\n");
  const original = Object.values(problem.starter_files).join("\n");
  const changed = code !== original;

  // Cart problem: detect if fix is applied (round() or Decimal)
  const isCartProblem = problem.id === "prob-cart-float-001";
  let testResults: Record<string, "pass" | "fail"> = {};

  if (isCartProblem) {
    const cartCode = submission.submitted_files["cart.py"] ?? "";
    const fixed = /\bround\s*\(/.test(cartCode) || /Decimal/.test(cartCode);
    testResults = {
      test_single_item: "pass",
      test_empty_cart: "pass",
      test_floating_point_drift: fixed ? "pass" : "fail",
      test_mixed_items: "pass",
    };
    const passed = Object.values(testResults).filter((v) => v === "pass").length;
    const total = Object.values(testResults).length;
    const passRate = passed / total;
    let score: number;
    let reasoning: string;
    if (!changed) {
      score = 1.0;
      reasoning = "No edits detected versus the starter files. The floating-point drift test continues to fail.";
    } else if (fixed) {
      score = 4.5;
      reasoning = "All four tests pass. Implementation honours the docstring contract by rounding to two decimal places.";
    } else {
      score = 2.0;
      reasoning = "Code was modified but the precision drift test still fails. The fix does not match the docstring contract.";
    }
    return { score, reasoning, testResults, passRate };
  }

  // Generic fallback
  const score = changed ? 3.0 : 1.5;
  const passRate = changed ? 0.75 : 0.25;
  testResults = { generic_test: changed ? "pass" : "fail" };
  return {
    score,
    reasoning: changed ? "Submission diverges from starter; partial pass on heuristic check." : "No changes detected.",
    testResults,
    passRate,
  };
}

function gradeAiCollaboration(submission: Submission): { score: number; reasoning: string; nudges: number } {
  const prompts = submission.prompt_history;
  const promptCount = submission.num_prompts;
  const lower = prompts.toLowerCase();

  let score = 2.5;
  const reasons: string[] = [];

  if (promptCount === 0) {
    return {
      score: 1.0,
      reasoning: "No prompt history captured. Without evidence of how AI was directed, collaboration cannot be assessed positively.",
      nudges: 0,
    };
  }

  if (promptCount >= 3) {
    score += 0.5;
    reasons.push("multi-step prompt sequence");
  }
  if (/no, instead|actually|let's not|undo|revert|don't/i.test(prompts)) {
    score += 0.7;
    reasons.push("explicit course corrections to AI");
  }
  if (/why|explain|walk me through/i.test(prompts)) {
    score += 0.4;
    reasons.push("asked AI for reasoning");
  }
  if (/diagnose|what could cause|investigate/i.test(prompts)) {
    score += 0.4;
    reasons.push("started with diagnose intent rather than fix-this dump");
  }
  if (prompts.length > 200) {
    score += 0.3;
    reasons.push("substantive prompt content");
  }
  if (prompts.length < 30) {
    score -= 0.5;
    reasons.push("very short prompts limit signal");
  }

  const nudges = (lower.match(/no,|actually|instead|wait|hmm|undo/g) ?? []).length;

  return {
    score: clamp(score, 1, 5),
    reasoning: reasons.length > 0 ? `Signals: ${reasons.join("; ")}.` : "Limited signal in prompt history.",
    nudges,
  };
}

function gradeVerification(submission: Submission, passRate: number): { score: number; reasoning: string } {
  const prompts = submission.prompt_history;
  const mentionsTesting = /\btest|\bverify|\bcheck|\bassert|edge case|run the test/i.test(prompts);

  let score = 2.5;
  if (passRate >= 0.95) score += 1.0;
  else if (passRate >= 0.7) score += 0.4;
  if (mentionsTesting) score += 0.8;

  const reasoning = [
    `Test pass rate ${(passRate * 100).toFixed(0)}%.`,
    mentionsTesting ? "Prompt history references running tests or checking edges." : "No explicit verification mentions in prompt history.",
  ].join(" ");

  return { score: clamp(score, 1, 5), reasoning };
}

function gradeDecomposition(submission: Submission): { score: number; reasoning: string } {
  const prompts = submission.prompt_history;
  let score = 2.5;
  const signals: string[] = [];

  if (/\bplan\b|\bsteps?\b|\bfirst,? then/i.test(prompts)) {
    score += 0.7;
    signals.push("plan-first language");
  }
  if (/clarify|what should happen if|what's the expected/i.test(prompts)) {
    score += 0.7;
    signals.push("clarifying questions");
  }
  if (/\b1\.\s|\b2\.\s/.test(prompts)) {
    score += 0.4;
    signals.push("explicit numbered breakdown");
  }
  if (prompts.length < 50) score -= 0.5;

  return {
    score: clamp(score, 1, 5),
    reasoning: signals.length > 0 ? `Signals: ${signals.join("; ")}.` : "Limited decomposition signal in prompt history.",
  };
}

function gradeCommunication(submission: Submission): { score: number; reasoning: string } {
  const prompts = submission.prompt_history;
  let score = 2.5;
  const signals: string[] = [];

  if (/because|so that|since|the reason/i.test(prompts)) {
    score += 0.8;
    signals.push("decision rationale present");
  }
  if (prompts.length > 300) {
    score += 0.4;
    signals.push("substantive narration length");
  }
  if (/trade-?off|alternative|considered/i.test(prompts)) {
    score += 0.6;
    signals.push("trade-off discussion");
  }

  return {
    score: clamp(score, 1, 5),
    reasoning: signals.length > 0 ? `Signals: ${signals.join("; ")}.` : "Sparse narration; decisions opaque.",
  };
}

function detectAntiPatterns(submission: Submission): AntiPatternBundle {
  const prompts = submission.prompt_history;
  const promptCount = submission.num_prompts;
  const lower = prompts.toLowerCase();

  const handsOff: AntiPatternFlag = {
    triggered: promptCount <= 1 && prompts.length > 0,
    evidence:
      promptCount <= 1 && prompts.length > 0
        ? "Single prompt with no follow-up review or correction."
        : "",
  };
  const featureMarathon: AntiPatternFlag = {
    triggered: promptCount > 12,
    evidence: promptCount > 12 ? `High prompt rate (${promptCount}) without proportional review evidence.` : "",
  };
  const aiShowcase: AntiPatternFlag = {
    triggered: /showcase|demo my|let me show|advanced technique/i.test(prompts),
    evidence: /showcase|demo my|let me show|advanced technique/i.test(prompts) ? "Prompt mentions demonstrating tooling rather than solving problem." : "",
  };
  const notThinking: AntiPatternFlag = {
    triggered: /just fix it|rewrite this|do whatever/i.test(lower),
    evidence: /just fix it|rewrite this|do whatever/i.test(lower) ? "Prompt delegates planning entirely to AI." : "",
  };

  return { hands_off: handsOff, feature_marathon: featureMarathon, ai_showcase: aiShowcase, not_thinking: notThinking };
}

export function gradeSubmission(submission: Submission, problem: Problem): {
  score: ScoreBreakdown;
  antiPatterns: AntiPatternBundle;
  testResults: Record<string, "pass" | "fail">;
  testPassRate: number;
} {
  const correctness = gradeCorrectness(submission, problem);
  const aiCollabRaw = gradeAiCollaboration(submission);
  const verification = gradeVerification(submission, correctness.passRate);
  const decomposition = gradeDecomposition(submission);
  const communication = gradeCommunication(submission);
  const antiPatterns = detectAntiPatterns(submission);

  const totalPenalty = (Object.keys(antiPatterns) as Array<keyof AntiPatternBundle>)
    .filter((k) => antiPatterns[k].triggered)
    .reduce((sum, k) => sum + ANTI_PATTERN_PENALTY[k], 0);
  const aiCollab = clamp(aiCollabRaw.score - totalPenalty, 1, 5);
  const aiCollabReasoning =
    totalPenalty > 0
      ? `${aiCollabRaw.reasoning} Anti-pattern penalty applied: -${totalPenalty.toFixed(1)}.`
      : aiCollabRaw.reasoning;

  const weights = WEIGHTS[problem.difficulty];
  const dimScores: Record<string, number | null> = {
    correctness: correctness.score,
    problem_decomposition: weights.problem_decomposition > 0 ? decomposition.score : null,
    ai_collaboration: aiCollab,
    verification_quality: weights.verification_quality > 0 ? verification.score : null,
    communication: weights.communication > 0 ? communication.score : null,
  };

  let weightedSum = 0;
  for (const dim of Object.keys(weights)) {
    const s = dimScores[dim];
    if (s !== null && s !== undefined) {
      weightedSum += s * weights[dim];
    }
  }
  const total = round2(weightedSum * 20);

  const score: ScoreBreakdown = {
    correctness: { score: round2(correctness.score), reasoning: correctness.reasoning },
    problem_decomposition: {
      score: dimScores.problem_decomposition !== null ? round2(decomposition.score) : null,
      reasoning: weights.problem_decomposition > 0 ? decomposition.reasoning : "Not evaluated for this difficulty.",
    },
    ai_collaboration: { score: round2(aiCollab), reasoning: aiCollabReasoning },
    verification_quality: {
      score: dimScores.verification_quality !== null ? round2(verification.score) : null,
      reasoning: weights.verification_quality > 0 ? verification.reasoning : "Not evaluated for this difficulty.",
    },
    communication: {
      score: dimScores.communication !== null ? round2(communication.score) : null,
      reasoning: weights.communication > 0 ? communication.reasoning : "Not evaluated for this difficulty.",
    },
    total,
    weights_applied: weights,
    judge_model: "mock-grader-v0",
  };

  return {
    score,
    antiPatterns,
    testResults: correctness.testResults,
    testPassRate: round2(correctness.passRate),
  };
}
