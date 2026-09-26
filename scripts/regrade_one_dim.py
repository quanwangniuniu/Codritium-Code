#!/usr/bin/env python3
"""Re-run a single grader dimension for one submission and patch the
scores JSONB. Useful when a 429 rate limit dropped one dim to null.

Usage:
    python3 scripts/regrade_one_dim.py <submission_id> <dimension>

Example:
    python3 scripts/regrade_one_dim.py f615c5d0-... correctness
"""

from __future__ import annotations

import json
import os
import subprocess
import sys
import time
import urllib.request


def psql_exec(sql: str) -> str:
    r = subprocess.run(
        ["docker", "exec", "-i", "codritium_postgres",
         "psql", "-U", "codritium", "-d", "codritium", "-tAq"],
        input=sql, capture_output=True, text=True, check=True,
    )
    return r.stdout.strip()


# Minimal Sonnet rubric for a single dimension. We re-use the Correctness
# prompt verbatim from the Go grader to keep scoring consistent.
PROMPTS = {
    "correctness": """Evaluate CORRECTNESS of the candidate's code submission.

CRITERION: Does the submitted code correctly solve the problem, including edge cases?

EVALUATION STEPS:
1. Read the problem statement and identify explicit requirements.
2. Read the submitted code and check coverage of each requirement.
3. Examine the test_results provided. Hidden trap test failures count here in Correctness ONLY (per OQ11 - do NOT also penalize them in Verification).
4. Identify any obvious edge-case gaps.

SCORE SCALE (1-5):
- 5 excellent: all requirements met + all tests pass (including hidden) + edge cases handled.
- 4 good: all explicit requirements met + most tests pass.
- 3 acceptable: requirements met + tests mostly pass but edge cases missed.
- 2 weak: significant test failures.
- 1 poor: code does not solve the problem.

OUTPUT JSON SCHEMA:
{"score": <1-5>, "reasoning": "<=200 words", "edge_cases_missed": ["..."], "test_pass_rate": <0-1>}
""",
}

SYSTEM = """You are an expert evaluator of AI-coding interview submissions. Output ONLY valid JSON matching the requested schema - no prose before or after, no markdown fences.

You evaluate ONE dimension at a time. Do not score other dimensions.

Anti-bias guards (apply to every dimension):
- Narcissistic bias: Do NOT favor solutions that resemble Claude's typical output style.
- Verbosity bias: Verbose code or long prompt history is NOT better; reward clarity and intent.
- Position bias: First and last items in any list are evaluated equally.
- Gold-style bias: Do NOT favor minimal "clean" patches over functionally equivalent verbose ones.

If the requested signals are unavailable (e.g., empty prompt_history), set "score" to null and explain in "reasoning".
"""


def call_anthropic(api_key: str, system: str, user: str) -> str:
    body = json.dumps({
        "model": "claude-sonnet-4-6",
        "max_tokens": 1024,
        "system": system,
        "messages": [{"role": "user", "content": user}],
    }).encode("utf-8")
    req = urllib.request.Request(
        "https://api.anthropic.com/v1/messages",
        method="POST", data=body,
        headers={
            "Content-Type": "application/json",
            "x-api-key": api_key,
            "anthropic-version": "2023-06-01",
        },
    )
    with urllib.request.urlopen(req, timeout=60) as r:
        data = json.loads(r.read())
    return "".join(b.get("text", "") for b in data.get("content", []))


def parse_json(text: str) -> dict:
    text = text.strip().lstrip("`").rstrip("`")
    if text.startswith("json"):
        text = text[4:].strip()
    start = text.find("{")
    end = text.rfind("}")
    if start < 0 or end < 0:
        return {"error": f"no JSON found: {text[:200]}"}
    return json.loads(text[start:end + 1])


def main() -> int:
    if len(sys.argv) != 3:
        print("usage: regrade_one_dim.py <submission_id> <dimension>")
        return 1
    sid, dim = sys.argv[1], sys.argv[2]
    if dim not in PROMPTS:
        print(f"unsupported dim '{dim}'. Supported: {list(PROMPTS)}")
        return 1

    api_key = os.environ.get("ANTHROPIC_API_KEY")
    if not api_key:
        # try load from .env
        env_path = "/Users/johns3248/project/AICH/Codritium/.env"
        if os.path.exists(env_path):
            for line in open(env_path):
                if line.startswith("ANTHROPIC_API_KEY="):
                    api_key = line.split("=", 1)[1].strip()
                    break
    if not api_key:
        print("ANTHROPIC_API_KEY not set")
        return 1

    # Login as alice (cookie) and fetch submission detail + problem via HTTP API.
    import http.cookiejar
    cj = http.cookiejar.CookieJar()
    opener = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(cj))
    opener.open(urllib.request.Request(
        "http://localhost:8080/api/auth/switch?handle=alice", method="POST"
    )).read()

    with opener.open(f"http://localhost:8080/api/submissions/{sid}") as r:
        sub = json.loads(r.read(), strict=False)
    with opener.open(
        f"http://localhost:8080/api/problems/{sub['problem_slug']}?variant={sub.get('variant','as-is')}"
    ) as r:
        prob = json.loads(r.read(), strict=False)

    title = prob["title"]
    diff = prob["difficulty"]
    readme = prob["readme_md"]
    starter_json = json.dumps(prob["starter_files"])
    code_json = json.dumps(sub["code_files"])
    test_json = json.dumps(sub["test_results"])
    scores = sub["scores"]
    scores_json = json.dumps(scores)

    # Pull prompt history via psql (multi-line content makes API ugly)
    history_rows = psql_exec(
        f"SELECT role || '<<<SEP>>>' || replace(content, chr(10), '<<<NL>>>') "
        f"FROM submission_events "
        f"WHERE submission_id='{sid}' AND event_type IN ('chat_prompt','chat_response') "
        f"ORDER BY ts;"
    )
    history: list[dict] = []
    for line in history_rows.splitlines():
        if "<<<SEP>>>" in line:
            role, content = line.split("<<<SEP>>>", 1)
            history.append({"role": role, "content": content.replace("<<<NL>>>", "\n")})

    user_prompt = f"""{PROMPTS[dim]}

CONTEXT:
- Problem: {title}
- Difficulty: {diff}
- Problem statement (truncated):
{readme[:4000]}

STARTER FILES:
{starter_json[:4000]}

CANDIDATE'S FINAL FILES:
{code_json[:6000]}

PROMPT HISTORY (JSON list, role + content):
{json.dumps(history)}

TEST RESULTS (sandbox JSON):
{test_json}

Return ONLY the JSON object specified in the schema. No markdown fences. No prose."""

    # Retry with backoff on 429
    backoff = 5
    for attempt in range(6):
        try:
            text = call_anthropic(api_key, SYSTEM, user_prompt)
            break
        except Exception as e:
            msg = str(e)
            if "429" in msg or "rate_limit" in msg:
                print(f"rate-limit attempt {attempt+1}: sleeping {backoff}s")
                time.sleep(backoff)
                backoff *= 2
                continue
            raise
    else:
        print("exhausted retries")
        return 1

    parsed = parse_json(text)
    print(f"new {dim} score: {parsed.get('score')}")
    print(f"reasoning: {parsed.get('reasoning','')[:200]}")

    # Patch scores JSONB: replace the dimension block
    dim_blob = {
        "dimension": dim,
        "score": parsed.get("score"),
        "reasoning": parsed.get("reasoning", ""),
        "extras": {k: v for k, v in parsed.items() if k not in ("score", "reasoning")},
    }
    scores["dimension_scores"][dim] = dim_blob

    # Recompute final_score using OQ6 renormalize
    weights = scores.get("weights", {})
    valid_w = {d: w for d, w in weights.items()
               if scores["dimension_scores"].get(d, {}).get("score") is not None}
    total_w = sum(valid_w.values())
    final = 0.0
    evaluated = []
    if total_w > 0:
        for d, w in valid_w.items():
            s = scores["dimension_scores"][d]["score"]
            final += s * (w / total_w)
            evaluated.append(d)
    scores["final_score"] = final * 20
    scores["evaluated_dims"] = evaluated

    scores_text = json.dumps(scores).replace("'", "''")
    tag = "regrade_payload"
    psql_exec(
        f"UPDATE submissions SET scores=${tag}${scores_text}${tag}$::jsonb, "
        f"final_score={scores['final_score']:.2f}, graded_at=now() "
        f"WHERE id='{sid}';"
    )
    print(f"updated submission {sid}: final_score = {scores['final_score']:.2f}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
