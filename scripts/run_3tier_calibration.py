#!/usr/bin/env python3
"""Run the 3-tier calibration submissions (poor / mid / good).

For each tier, pre-create a submission via SQL, inject the prompt_history
events from temp/22/<tier>/prompt_history.txt, compose code_files from
starter + tier-specific rate_limiter.py, then POST to /api/submissions with
the existing submission_id to trigger the grader pipeline.

Prints the submission_id per tier and waits for grading to finish.
"""

from __future__ import annotations

import json
import subprocess
import sys
import time
from pathlib import Path

import urllib.request
import urllib.parse
import http.cookiejar

API = "http://localhost:8080"
TEMP_DIR = Path("/Users/johns3248/project/AICH/temp/22")
TIERS = ["poor", "mid", "good"]


def psql_exec(sql: str) -> str:
    r = subprocess.run(
        ["docker", "exec", "-i", "codritium_postgres",
         "psql", "-U", "codritium", "-d", "codritium", "-tAq"],
        input=sql, capture_output=True, text=True, check=True,
    )
    return r.stdout.strip()


def parse_history(path: Path) -> list[tuple[str, str]]:
    text = path.read_text(encoding="utf-8")
    events: list[tuple[str, str]] = []
    role: str | None = None
    buf: list[str] = []
    for line in text.splitlines():
        stripped = line.strip()
        if stripped == "=== Conversation export ===":
            continue
        if stripped == "=== end ===":
            break
        if line.startswith("model:") or line.startswith("exported:"):
            continue
        if stripped == "USER:" or line.startswith("USER:"):
            if role:
                events.append((role, "\n".join(buf).strip()))
            role = "user"
            buf = []
            continue
        if stripped == "A:" or line.startswith("A:"):
            if role:
                events.append((role, "\n".join(buf).strip()))
            role = "assistant"
            buf = []
            continue
        if role:
            buf.append(line)
    if role:
        events.append((role, "\n".join(buf).strip()))
    return [(r, c) for r, c in events if c]


def make_opener() -> urllib.request.OpenerDirector:
    cj = http.cookiejar.CookieJar()
    return urllib.request.build_opener(urllib.request.HTTPCookieProcessor(cj))


def login(opener, handle: str) -> None:
    req = urllib.request.Request(
        f"{API}/api/auth/switch?handle={handle}", method="POST"
    )
    opener.open(req).read()


def get_problem(opener, slug: str) -> dict:
    with opener.open(f"{API}/api/problems/{slug}?variant=as-is") as r:
        return json.loads(r.read())


def post_submit(opener, body: dict) -> dict:
    data = json.dumps(body).encode("utf-8")
    req = urllib.request.Request(
        f"{API}/api/submissions", method="POST", data=data,
        headers={"Content-Type": "application/json"},
    )
    with opener.open(req) as r:
        return json.loads(r.read())


def get_submission(opener, sid: str) -> dict:
    with opener.open(f"{API}/api/submissions/{sid}") as r:
        raw = r.read().decode("utf-8")
        return json.loads(raw, strict=False)


def main() -> int:
    opener = make_opener()
    login(opener, "alice")

    problem = get_problem(opener, "22-build-rate-limiter-middleware")
    starter = problem["starter_files"]

    submission_ids: dict[str, str] = {}

    for tier in TIERS:
        print(f"\n=== {tier.upper()} ===")
        # 1. Pre-create submission row
        sid = psql_exec(
            "INSERT INTO submissions (user_id, problem_id, status, variant) "
            "SELECT u.id, p.id, 'pending', 'as-is' "
            "FROM users u, problems p "
            "WHERE u.handle='alice' AND p.slug='22-build-rate-limiter-middleware' "
            "RETURNING id;"
        )
        print(f"submission_id: {sid}")

        # 2. Parse + inject events
        events = parse_history(TEMP_DIR / tier / "prompt_history.txt")
        print(f"events parsed: {len(events)}")
        sql_lines: list[str] = []
        for idx, (role, content) in enumerate(events):
            evtype = "chat_prompt" if role == "user" else "chat_response"
            tag = f"codritium_{tier}_{idx}"
            sql_lines.append(
                f"INSERT INTO submission_events (submission_id, event_type, role, content) "
                f"VALUES ('{sid}', '{evtype}', '{role}', ${tag}${content}${tag}$);"
            )
        if sql_lines:
            psql_exec("\n".join(sql_lines))

        # 3. Compose code_files
        fixed = (TEMP_DIR / tier / "fixed" / "rate_limiter.py").read_text(encoding="utf-8")
        code_files = dict(starter)
        code_files["rate_limiter.py"] = fixed
        print(f"code_files: {list(code_files.keys())} (rate_limiter.py {len(fixed)} bytes)")

        # 4. POST submit to trigger grader
        resp = post_submit(opener, {
            "problem_slug": "22-build-rate-limiter-middleware",
            "variant": "as-is",
            "submission_id": sid,
            "code_files": code_files,
        })
        print(f"submit response: {resp}")
        submission_ids[tier] = sid

    # 5. Wait for all 3 to finish
    print("\n=== Polling all 3 submissions ===")
    pending = dict(submission_ids)
    results: dict[str, dict] = {}
    deadline = time.time() + 300
    while pending and time.time() < deadline:
        for tier in list(pending):
            sid = pending[tier]
            try:
                d = get_submission(opener, sid)
            except Exception as e:
                print(f"{tier} poll error: {e}")
                continue
            if d.get("status") in ("graded", "failed"):
                results[tier] = d
                print(f"{tier}: status={d['status']} final_score={d.get('final_score')}")
                del pending[tier]
        if pending:
            time.sleep(5)

    print("\n=== Final scores ===")
    for tier in TIERS:
        d = results.get(tier)
        if not d:
            print(f"{tier}: STILL PENDING (timeout)")
            continue
        print(f"\n--- {tier.upper()} (id={submission_ids[tier]}) ---")
        print(f"status: {d['status']}")
        print(f"final_score: {d.get('final_score')}")
        tr = d.get("test_results") or {}
        if "pass_count" in tr:
            print(f"tests: {tr['pass_count']}/{tr.get('total')} passed")
        scores = d.get("scores") or {}
        for dim, ds in (scores.get("dimension_scores") or {}).items():
            score = ds.get("score")
            reasoning = (ds.get("reasoning") or ds.get("error") or "")[:160]
            print(f"  {dim}: {score}/5  — {reasoning}")
        print(f"URL: http://localhost:3000/submissions/{submission_ids[tier]}")

    return 0


if __name__ == "__main__":
    sys.exit(main())
