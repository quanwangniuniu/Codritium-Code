package grader

// Rubric prompts for the 5 G-Eval dimensions. Each prompt is independent —
// per Anthropic's "isolated LLM-as-judge per dimension" guidance.
// Anti-bias guards (narcissistic / verbosity / position / gold-style) are
// folded into every prompt.

const rubricSystem = `You are an expert evaluator of AI-coding interview submissions. Output ONLY valid JSON matching the requested schema — no prose before or after, no markdown fences.

You evaluate ONE dimension at a time. Do not score other dimensions.

Anti-bias guards (apply to every dimension):
- Narcissistic bias: Do NOT favor solutions that resemble Claude's typical output style.
- Verbosity bias: Verbose code or long prompt history is NOT better; reward clarity and intent.
- Position bias: First and last items in any list are evaluated equally.
- Gold-style bias: Do NOT favor minimal "clean" patches over functionally equivalent verbose ones.

If the requested signals are unavailable (e.g., empty prompt_history), set "score" to null and explain in "reasoning".`

type DimensionSpec struct {
	Name       string
	Prompt     string
}

var Dimensions = []DimensionSpec{
	{
		Name: "correctness",
		Prompt: `Evaluate CORRECTNESS of the candidate's code submission.

CRITERION: Does the submitted code correctly solve the problem, including edge cases?

EVALUATION STEPS:
1. Read the problem statement and identify explicit requirements.
2. Read the submitted code and check coverage of each requirement.
3. Examine the test_results provided. Hidden trap test failures count here in Correctness ONLY (per OQ11 — do NOT also penalize them in Verification).
4. Identify any obvious edge-case gaps.

HARD CONSTRAINT — sandbox is the source of truth for test pass rate:
- When test_results is present and contains pass_count + total, set test_pass_rate = pass_count / total exactly. Do NOT estimate or infer from code.
- When test_results is null, set test_pass_rate to null and note in reasoning that the sandbox did not run.
- The reasoning sentence describing how many tests pass MUST match pass_count / total verbatim (e.g. "3/4 tests pass" when pass_count=3, total=4). Do not contradict the sandbox.

SCORE FLOOR FROM SANDBOX:
- test_pass_rate == 0 forces score = 1. Zero tests passing is a "code does not solve the problem" outcome regardless of prose, structure, or design intent in the patch.
- test_pass_rate < 0.5 caps score at 2.
- test_pass_rate < 1.0 caps score at 3.

SCORE SCALE (1-5):
- 5 excellent: all requirements met + all tests pass (including hidden) + edge cases handled.
- 4 good: all explicit requirements met + most tests pass.
- 3 acceptable: requirements met + tests mostly pass but edge cases missed.
- 2 weak: significant test failures.
- 1 poor: code does not solve the problem.

BOUNDARIES FOR THIS DIMENSION:
Correctness measures ONLY: sandbox test pass rate + behavioral correctness of submitted code under hidden tests. Code hygiene (dead code removal, naming, organization) is NOT Correctness — score that in Verification instead.

OUTPUT JSON SCHEMA:
{"score": <1-5>, "reasoning": "<<=200 words>", "edge_cases_missed": ["..."], "test_pass_rate": <float 0-1 | null>}`,
	},
	{
		Name: "problem_decomposition",
		Prompt: `Evaluate PROBLEM DECOMPOSITION (only applies to Hard problems — return null for Easy/Medium).

CRITERION: Did the candidate break the problem into sub-problems and form a plan BEFORE prompting the AI?

EVALUATION STEPS:
1. If prompt_history is empty/missing, return score: null with reasoning "cannot evaluate without prompt history".
2. If difficulty is "easy" or "medium", return score: null with reasoning "dimension not applicable below Hard".
3. Examine the first 3-5 prompts; classify intent (diagnose / explore / fix / implement).
4. Check for any plan-file (AGENTS.md, scratch.md) created BEFORE first prompt.
5. Did the candidate ask clarifying questions about ambiguous spec?

SCORE SCALE:
- 5 excellent: clear written plan + explicit sub-problems + correct priority + clarifying questions.
- 4 good: plan present + most sub-problems identified.
- 3 acceptable: implicit plan-like behavior.
- 2 weak: jumped to prompting without a plan.
- 1 poor: pasted the entire problem to AI without forming own plan.

BOUNDARIES FOR THIS DIMENSION:
Problem Decomposition measures ONLY the candidate's plan articulation BEFORE asking AI. It is NOT about the final structure of the code (that is implicit in Correctness) and NOT about how the AI replied (that is AI Collaboration).

OUTPUT JSON SCHEMA:
{"score": <1-5 | null>, "reasoning": "<<=200 words>", "first_prompt_intent": "diagnose|explore|fix|implement|unknown", "clarifying_questions_count": <int | null>}`,
	},
	{
		Name: "ai_collaboration",
		Prompt: `Evaluate AI COLLABORATION ability.

CRITERION: Did the candidate DIRECT the AI, or did the AI direct the candidate? The candidate should be the senior engineer; the AI is the fast-but-fallible junior pair.

EVALUATION STEPS:
1. If prompt_history is empty/missing, return score: null with reasoning "cannot evaluate without prompt history".
2. Count "nudges" (small targeted corrections to AI output).
3. Look for selective accept/reject (not just "ok thanks").
4. Did the candidate push back when AI proposed off-plan?
5. Does the final code show signs of human modification (vs raw AI dump)?

SCORE SCALE:
- 5 excellent: plan → prompt → review → selective accept → iterate, with multiple nudges.
- 4 good: reviews each output, some selective undo.
- 3 acceptable: reviews but mostly accepts wholesale.
- 2 weak: accepts without review.
- 1 poor: Hands-Off — pasted problem to AI, accepted verbatim.

BOUNDARIES FOR THIS DIMENSION:
AI Collaboration measures ONLY the candidate's directive nudges, pushback frequency, and accept/reject/modify pattern on AI proposals. It is NOT about the quality of the AI's response itself (that is implicit in Correctness if the candidate accepted it).

Auto-approved tool uses (e.g. FileRead with auto:true in events) are runtime conveniences and carry NO candidate signal — exclude them from the nudges_count and the accept/reject pattern analysis.

OUTPUT JSON SCHEMA:
{"score": <1-5 | null>, "reasoning": "<<=200 words>", "nudges_count": <int | null>, "architectural_decisions_owner": "candidate|ai|mixed|unknown"}`,
	},
	{
		Name: "verification",
		Prompt: `Evaluate VERIFICATION & QUALITY.

CRITERION: Did the candidate review and test their own work?
IMPORTANT (per OQ11): This dimension scores ONLY user-visible verification artifacts. Do NOT penalize hidden grader test failures here — those are scored in Correctness only.

EVALUATION STEPS:
1. Check user-visible verification artifacts:
   a. Verbs in prompt_history: "ran tests", "verified", "checked", "asserted", "tested" (keyword detection).
   b. Manual scenario walkthroughs (e.g., explicit "I tried inputs X, Y, got Z").
   c. Contract self-check tables (≥3 named contract bullets with pass/fail).
   d. Invariant comments naming what was preserved.
2. Static code quality: naming, structure, error handling, edge case handling.

SCORE SCALE (per OQ12 partial credit rules):
- 5 excellent: keyword hits AND ≥2 artifact types AND clean code quality.
- 4 good (OQ12 partial-credit path): keyword fails BUT ≥2 artifact types AND clean code (cap at 4 — verbalized verification has stronger signal than artifacts alone).
- 3 acceptable: keyword OR ≥1 artifact + minor quality issues.
- 2 weak: no keyword + no artifact + quality issues.
- 1 poor: no verification of any kind + obvious quality problems. NEVER zero — absence of artifact ≠ definitive failure.

DO NOT count hidden grader test failures in this dimension (OQ11).

ABANDONMENT CHECK (hard rule):
If the candidate observed a verification failure (e.g. tests they ran came back red, sandbox surfaced an error) AND submitted without iterating to fix or explicitly justify the failure, verification MUST be capped at 2. Running a check and ignoring its result is anti-verification; it is worse than not checking, because it shows the candidate is performing verification rituals without using the signal.

BOUNDARIES FOR THIS DIMENSION:
Verification measures: artifact creation (tests / invariants / scenario walkthroughs) AND code hygiene (cleanup, dead code removal, naming, organization). Code hygiene that does not affect Correctness scoring belongs HERE, not in Correctness.

OUTPUT JSON SCHEMA:
{"score": <1-5>, "reasoning": "<<=200 words>", "keyword_detection_hit": <bool>, "artifact_types_present": ["contract_table"|"manual_scenario"|"invariant_comments"], "oq12_partial_credit_applied": <bool>}`,
	},
	{
		Name: "communication",
		Prompt: `Evaluate COMMUNICATION (only applies to Hard problems — return null for Easy/Medium).

CRITERION: Did the candidate explain their thinking, decisions, and trade-offs before/while coding?

EVALUATION STEPS:
1. If difficulty is "easy" or "medium", return score: null.
2. If prompt_history AND code_comments AND scratch_files are all empty, return null with reasoning "no narration channel".
3. Look for "before-prompt narration": explaining what they're about to do BEFORE prompting.
4. Look for "decision explanations" in comments/scratch.
5. Trade-off discussion: weighing alternatives explicitly.

SCORE SCALE:
- 5 excellent: clear narration before each major prompt + decisions explained + trade-offs.
- 4 good: narration on most decisions.
- 3 acceptable: some narration.
- 2 weak: sparse narration.
- 1 poor: no narration.
- null: no narration channel available.

BOUNDARIES FOR THIS DIMENSION:
Communication measures ONLY narration, decision explanation, and trade-off discussion appearing in prompt_history or scratch files. Pure code comments alone are NOT sufficient signal — they read as documentation, not communication of reasoning to a collaborator.

OUTPUT JSON SCHEMA:
{"score": <1-5 | null>, "reasoning": "<<=200 words>", "before_prompt_narration_count": <int | null>, "decision_explanations_count": <int | null>}`,
	},
}

// DifficultyWeights returns the dimension weight per difficulty.
// Easy: only Correctness + AI Collaboration.
// Medium: + Verification.
// Hard: full 5 dimensions.
func DifficultyWeights(difficulty string) map[string]float64 {
	switch difficulty {
	case "easy":
		return map[string]float64{
			"correctness":      0.70,
			"ai_collaboration": 0.30,
		}
	case "medium":
		return map[string]float64{
			"correctness":      0.50,
			"ai_collaboration": 0.30,
			"verification":     0.20,
		}
	case "hard":
		return map[string]float64{
			"correctness":           0.25,
			"problem_decomposition": 0.25,
			"ai_collaboration":      0.25,
			"verification":          0.15,
			"communication":         0.10,
		}
	default:
		return map[string]float64{
			"correctness":      0.50,
			"ai_collaboration": 0.30,
			"verification":     0.20,
		}
	}
}
