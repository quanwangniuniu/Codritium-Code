#!/usr/bin/env python3
"""E2B sandbox wrapper: spawn a sandbox, drop candidate files + hidden test,
run pytest, return JSON results to stdout.

Invoked by the Go backend submission handler:

    cd Codritium/sandbox && uv run run_pytest.py <input.json>

Input JSON (stdin or first argv):
    {
      "starter_files": {"a.py": "...", "b.py": "..."},
      "candidate_files": {"a.py": "...", "b.py": "..."},
      "hidden_test_filename": "test_answer.py",
      "hidden_test_content": "...",
      "timeout_sec": 60
    }

Output JSON (stdout):
    {
      "status": "ok" | "error",
      "test_results": [{"name": "...", "outcome": "passed|failed", "message": "..."}],
      "pass_count": int,
      "fail_count": int,
      "total": int,
      "stdout": "...",
      "stderr": "...",
      "error": "..." (if status=error)
    }
"""

from __future__ import annotations

import json
import os
import re
import sys
import time

try:
    from e2b import Sandbox
    from e2b.sandbox.commands.command_handle import CommandExitException
except ImportError as ie:
    print(json.dumps({"status": "error", "error": f"e2b package import failed: {ie}"}))
    sys.exit(1)


def parse_pytest_output(stdout: str) -> tuple[list[dict], int, int]:
    """Pull individual test outcomes out of pytest's '-v' report."""
    test_results = []
    pass_count = 0
    fail_count = 0
    # Lines like: tests/test_answer.py::test_basic PASSED [10%]
    line_re = re.compile(r"^(\S+::\S+)\s+(PASSED|FAILED|ERROR|SKIPPED)", re.M)
    for m in line_re.finditer(stdout):
        name, outcome = m.group(1), m.group(2).lower()
        if outcome in ("passed",):
            pass_count += 1
        elif outcome in ("failed", "error"):
            fail_count += 1
        test_results.append({"name": name, "outcome": outcome})
    return test_results, pass_count, fail_count


def main() -> int:
    if len(sys.argv) > 1:
        with open(sys.argv[1], "r", encoding="utf-8") as f:
            payload = json.load(f)
    else:
        payload = json.load(sys.stdin)

    candidate_files: dict[str, str] = payload["candidate_files"]
    hidden_test_filename: str = payload.get("hidden_test_filename", "test_answer.py")
    hidden_test_content: str = payload.get("hidden_test_content", "")
    timeout_sec: int = int(payload.get("timeout_sec", 60))

    api_key = os.environ.get("E2B_API_KEY")
    if not api_key:
        print(json.dumps({"status": "error", "error": "E2B_API_KEY not set"}))
        return 1

    started = time.time()

    try:
        # Use the default base template which includes Python 3.10+.
        sbx = Sandbox.create(timeout=timeout_sec + 30, api_key=api_key)
    except Exception as e:
        print(json.dumps({"status": "error", "error": f"create sandbox: {e}"}))
        return 1

    try:
        # Install pytest (idempotent — fast on subsequent calls in template-based sandboxes).
        install = sbx.commands.run("python -m pip install --quiet pytest", timeout=60)
        if install.exit_code != 0:
            print(
                json.dumps(
                    {
                        "status": "error",
                        "error": "pip install pytest failed",
                        "stdout": install.stdout,
                        "stderr": install.stderr,
                    }
                )
            )
            return 1

        workdir = "/home/user/workspace"
        sbx.commands.run(f"mkdir -p {workdir}", timeout=10)

        # Drop candidate files.
        for name, content in candidate_files.items():
            sbx.files.write(f"{workdir}/{name}", content)

        # Drop hidden test (NEVER part of starter_files).
        if hidden_test_content:
            sbx.files.write(f"{workdir}/{hidden_test_filename}", hidden_test_content)

        # Run pytest verbose with JUnit-style line markers we can parse.
        # pytest exits non-zero when any test fails, which e2b SDK raises as
        # CommandExitException. We want the structured output regardless.
        run_stdout = ""
        run_stderr = ""
        run_exit = 0
        try:
            run = sbx.commands.run(
                f"cd {workdir} && python -m pytest {hidden_test_filename} -v --tb=short --no-header",
                timeout=timeout_sec,
            )
            run_stdout = run.stdout
            run_stderr = run.stderr
            run_exit = run.exit_code
        except CommandExitException as ce:
            run_stdout = getattr(ce, "stdout", "") or ""
            run_stderr = getattr(ce, "stderr", "") or ""
            run_exit = getattr(ce, "exit_code", 1)

        test_results, pass_count, fail_count = parse_pytest_output(run_stdout)
        total = pass_count + fail_count

        result = {
            "status": "ok",
            "test_results": test_results,
            "pass_count": pass_count,
            "fail_count": fail_count,
            "total": total,
            "exit_code": run_exit,
            "stdout": run_stdout[-4000:],
            "stderr": run_stderr[-2000:],
            "duration_sec": round(time.time() - started, 2),
        }
        print(json.dumps(result))
        return 0
    except Exception as e:
        print(json.dumps({"status": "error", "error": f"run: {e}"}))
        return 1
    finally:
        try:
            sbx.kill()
        except Exception:
            pass


if __name__ == "__main__":
    sys.exit(main())
