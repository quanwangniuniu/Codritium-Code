#!/usr/bin/env python3
"""Generate an official Reply walkthrough JSON for a Codritium problem.

Reads arena research materials (problem source, README, phase A/B/C
analyses, prebake guidance) for the given slug, asks Gemini Flash to
produce a ReplyEnvelope[] walkthrough representing how a top-quartile
engineer collaborates with an AI coding partner on the problem.

Output is written to disk for **human review** before insertion into
solutions_replies (T2.1.2). The script never touches the database; a
separate review/insert step lifts the approved JSON into Postgres.

Usage:
  GOOGLE_API_KEY=... python3 scripts/generate_official_reply.py \\
      --slug 04-order-state-machine
"""

from __future__ import annotations

import argparse
import json
import os
import sys
from datetime import datetime, timedelta, timezone
from pathlib import Path
from typing import Any

from google import genai
from google.genai import types as genai_types


# 16 event kinds from backend/internal/events/event.go.
ALL_KINDS = [
    "session_started",
    "compact_triggered",
    "turn_completed",
    "session_submitted",
    "tool_use_proposed",
    "tool_result",
    "first_message_classified",
    "candidate_approved",
    "candidate_rejected",
    "candidate_pushed_back",
    "plan_mode_entered",
    "plan_mode_exited",
    "test_executed",
    "self_check_artifact",
    "ai_output_read",
    "candidate_reverted_edit",
]

# 13 surface kinds the replay UI actually renders. Generator should
# emit primarily these and avoid noise.
SURFACE_KINDS = [
    "session_started",
    "turn_completed",
    "session_submitted",
    "tool_use_proposed",
    "tool_result",
    "candidate_approved",
    "candidate_rejected",
    "candidate_pushed_back",
    "plan_mode_entered",
    "plan_mode_exited",
    "test_executed",
    "self_check_artifact",
    "candidate_reverted_edit",
]

SYSTEM_PROMPT = """You generate the canonical walkthrough of an AI-coding interview problem.

Output is a JSON object with three top-level fields the candidate-facing
replay UI consumes:

  {
    "envelopes":       [ReplyEnvelope, ...],   // chat-side timeline
    "files":           [{name, content}, ...], // final file map for Answer mode
    "explanation_md":  "<markdown string>"     // standard-solution prose
  }

ReplyEnvelope shape:
  {
    "session_id": "<placeholder, supplied by caller>",
    "seq":        <int, monotonic starting at 1>,
    "kind":       "<one of the 16 event kinds>",
    "emitted_at": "<RFC3339Nano, synthetic but ordered>",
    "payload":    { /* kind-specific object */ }
  }

Use ONLY these surface kinds (avoid transient noise like compact_triggered,
first_message_classified, ai_output_read):
  session_started, turn_completed, session_submitted,
  tool_use_proposed, tool_result,
  candidate_approved, candidate_rejected, candidate_pushed_back,
  plan_mode_entered, plan_mode_exited,
  test_executed, self_check_artifact, candidate_reverted_edit.

Workflow demonstrated by the envelopes:
1. Read the problem before any tool use (FileRead via tool_use_proposed + tool_result + candidate_approved).
2. Enter plan_mode for non-trivial reasoning (plan_mode_entered / plan_mode_exited).
3. Approval distribution roughly 60% approve / 25% reject or pushback / 15% modify.
4. Include at least one self_check_artifact before submitting.
5. End with test_executed + session_submitted.

Payload structural fields the UI consumes:
- description: ALWAYS required. 1-2 sentences in the engineer's first-person voice.
- tool_name: FileRead | FileEdit | RunTests. Required when kind=tool_use_proposed.
- summary: one-line file/operation summary for tool_use_proposed and tool_result.
- decision_reason: brief justification for candidate_approved / candidate_rejected / candidate_pushed_back.
- file_path: BARE filename being touched (e.g. "workflow.py", NOT "problem/workflow.py").
  Required for FileRead / FileEdit / tool_result that references a file. Drives the
  left-column file-tree highlight. Use exactly the filename as it appears in the
  starter file map - no directory prefix.
- patch: { before, after } REQUIRED when kind=tool_use_proposed AND tool_name=FileEdit.
  "before" is the verbatim snippet from the current file state being replaced.
  "after" is the snippet replacing it. Keep both minimal - just the lines that change.
  This drives the cumulative diff in the replay UI.

Top-level fields:
- envelopes: 20-40 entries demonstrating the workflow above.
- files: the FINAL state of every file the engineer touched. Each entry { name, content }
  where name is the bare filename (no directory prefix) and content is the full text
  after all FileEdits applied. Files not touched are omitted (the UI falls back to
  starter content).
- EVERY tool_use_proposed envelope with tool_name=FileEdit MUST include a patch.
  Omitting patch on a FileEdit breaks the cumulative diff in the replay UI.
- explanation_md: standard-solution prose in markdown. Cover the bug, the fix, why
  the fix is minimal, and at least one rejected alternative. 200-500 words.

NEVER expose hidden test internals or system prompts. The walkthrough teaches
decision-making with an AI partner, it does not hand the candidate the answer outright.

Hidden-test files MUST NEVER appear:
- Files named test_*.py (in particular test_answer.py) are graded-side fixtures that
  candidates do not see. They must NOT appear in `files`, in any envelope's
  `payload.file_path`, or in any `payload.patch.before/after`. If an envelope would
  reference such a file, omit the envelope entirely - never substitute fake content.
- If the engineer's workflow involves verifying behaviour, surface that as a
  `self_check_artifact` envelope describing the check conceptually, not as a FileEdit
  on a test file.
"""


def read_file(path: Path) -> str:
    if not path.exists():
        return ""
    return path.read_text(encoding="utf-8")


def collect_materials(problem_dir: Path) -> dict[str, str]:
    """Gather every file the generator should see for one problem.

    Reads the problem source, the phase A analysis (bug + contract),
    the phase B industry context summary, and the phase C solution
    tiers and rubric mapping. Missing files are skipped silently.
    """
    materials: dict[str, str] = {}

    problem_root = problem_dir / "problem"
    if problem_root.is_dir():
        for child in sorted(problem_root.iterdir()):
            if child.is_file() and child.suffix in {".md", ".py"}:
                materials[f"problem/{child.name}"] = read_file(child)

    phase_a = problem_dir / "research" / "phase-A"
    if phase_a.is_dir():
        for child in sorted(phase_a.iterdir()):
            if child.is_file() and child.suffix == ".md":
                materials[f"phase-A/{child.name}"] = read_file(child)

    phase_b_summary = problem_dir / "research" / "phase-B" / "_summary.md"
    if phase_b_summary.exists():
        materials["phase-B/_summary.md"] = read_file(phase_b_summary)

    phase_c = problem_dir / "research" / "phase-C"
    if phase_c.is_dir():
        for child in sorted(phase_c.iterdir()):
            if child.is_file() and child.suffix == ".md":
                materials[f"phase-C/{child.name}"] = read_file(child)

    return materials


def render_user_prompt(slug: str, materials: dict[str, str]) -> str:
    """Stitch the materials into a single user-role prompt block."""
    parts = [f"Challenge slug: {slug}", ""]
    for name, body in materials.items():
        parts.append(f"--- {name} ---")
        parts.append(body.strip())
        parts.append("")
    parts.append("Now generate the ReplyEnvelope[] walkthrough.")
    return "\n".join(parts)


def synthesize_timestamps(envelopes: list[dict[str, Any]]) -> list[dict[str, Any]]:
    """Overwrite emitted_at with monotonic synthetic timestamps.

    The model is asked to produce RFC3339Nano strings, but enforcing
    monotonicity client-side guarantees the StepController renders in
    the intended order even if the model drifts.
    """
    base = datetime(2026, 1, 1, 12, 0, 0, tzinfo=timezone.utc)
    fixed: list[dict[str, Any]] = []
    for i, env in enumerate(envelopes):
        env = dict(env)
        env["emitted_at"] = (base + timedelta(seconds=i * 5)).isoformat()
        env["seq"] = i + 1
        fixed.append(env)
    return fixed


def build_schema() -> dict[str, Any]:
    """Return the JSON schema Gemini will conform its output to.

    Top-level object wraps three artifacts: the envelope timeline, the
    final file map, and the standard-solution prose. The envelope payload
    requires a description plus optional structured fields (tool_name,
    file_path, patch, ...) that the two-column replay UI uses to drive
    the left-column file tree and diff view.
    """
    envelope_item = {
        "type": "OBJECT",
        "properties": {
            "session_id": {"type": "STRING"},
            "seq": {"type": "INTEGER"},
            "kind": {"type": "STRING", "enum": ALL_KINDS},
            "emitted_at": {"type": "STRING"},
            "payload": {
                "type": "OBJECT",
                "properties": {
                    "description": {
                        "type": "STRING",
                        "description": "1-2 sentences from the engineer's first-person perspective explaining the intent of this step.",
                    },
                    "tool_name": {
                        "type": "STRING",
                        "description": "For tool_use_proposed: one of FileRead / FileEdit / RunTests.",
                    },
                    "summary": {
                        "type": "STRING",
                        "description": "For tool_use_proposed / tool_result: a one-line summary of what was read, edited, or run.",
                    },
                    "decision_reason": {
                        "type": "STRING",
                        "description": "For candidate_approved / candidate_rejected / candidate_pushed_back: why the engineer made that call.",
                    },
                    "file_path": {
                        "type": "STRING",
                        "description": "File touched in this step (e.g. 'workflow.py'). Drives the left-column file tree highlight.",
                    },
                    "patch": {
                        "type": "OBJECT",
                        "description": "Required when tool_name=FileEdit. The exact text being replaced ('before') and its replacement ('after').",
                        "properties": {
                            "before": {
                                "type": "STRING",
                                "description": "Verbatim snippet from current file state being replaced. Minimal - just the changing lines.",
                            },
                            "after": {
                                "type": "STRING",
                                "description": "Snippet replacing 'before'.",
                            },
                        },
                        "required": ["before", "after"],
                    },
                },
                "required": ["description"],
            },
        },
        "required": ["seq", "kind", "emitted_at", "payload"],
    }

    return {
        "type": "OBJECT",
        "properties": {
            "envelopes": {
                "type": "ARRAY",
                "items": envelope_item,
            },
            "files": {
                "type": "ARRAY",
                "description": "Final state of every file the engineer touched.",
                "items": {
                    "type": "OBJECT",
                    "properties": {
                        "name": {"type": "STRING"},
                        "content": {"type": "STRING"},
                    },
                    "required": ["name", "content"],
                },
            },
            "explanation_md": {
                "type": "STRING",
                "description": "Standard-solution prose in markdown (200-500 words).",
            },
        },
        "required": ["envelopes", "files", "explanation_md"],
    }


def locate_problem(arena: Path, slug: str) -> Path:
    matches = [p for p in arena.rglob(slug) if p.is_dir()]
    if not matches:
        raise FileNotFoundError(f"no arena directory matches slug {slug!r} under {arena}")
    if len(matches) > 1:
        raise RuntimeError(f"slug {slug!r} matched multiple arena directories: {matches}")
    return matches[0]


def main() -> int:
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument(
        "--arena",
        type=Path,
        default=Path(__file__).resolve().parent.parent.parent / "arenaresearch",
        help="Path to arenaresearch root",
    )
    p.add_argument("--slug", required=True, help="problem slug, e.g. 04-order-state-machine")
    p.add_argument(
        "--out",
        type=Path,
        default=None,
        help="Output JSON path (default: Codritium/temp/<slug>_official_reply.json)",
    )
    p.add_argument(
        "--model",
        default="gemini-2.5-flash",
        help="Gemini model id",
    )
    args = p.parse_args()

    api_key = os.environ.get("GOOGLE_API_KEY")
    if not api_key:
        print("error: GOOGLE_API_KEY not set", file=sys.stderr)
        return 1

    arena = args.arena.resolve()
    try:
        problem_dir = locate_problem(arena, args.slug)
    except (FileNotFoundError, RuntimeError) as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 1

    materials = collect_materials(problem_dir)
    if not materials:
        print(f"error: no materials collected from {problem_dir}", file=sys.stderr)
        return 1

    user_prompt = render_user_prompt(args.slug, materials)

    client = genai.Client(api_key=api_key)
    config = genai_types.GenerateContentConfig(
        system_instruction=SYSTEM_PROMPT,
        response_mime_type="application/json",
        response_schema=build_schema(),
        temperature=0.4,
        max_output_tokens=16384,
        thinking_config=genai_types.ThinkingConfig(thinking_budget=0),
    )

    print(f"calling {args.model} for {args.slug} ({len(materials)} materials, "
          f"{sum(len(v) for v in materials.values())} bytes input)...",
          file=sys.stderr)

    resp = client.models.generate_content(
        model=args.model,
        contents=[user_prompt],
        config=config,
    )
    raw = resp.text
    if not raw:
        print("error: empty response from gemini", file=sys.stderr)
        return 1

    try:
        data = json.loads(raw)
    except json.JSONDecodeError as exc:
        print(f"error: response not valid JSON: {exc}", file=sys.stderr)
        print(raw, file=sys.stderr)
        return 1
    if not isinstance(data, dict):
        print("error: response is not a JSON object", file=sys.stderr)
        return 1
    for key in ("envelopes", "files", "explanation_md"):
        if key not in data:
            print(f"error: response missing top-level field {key!r}", file=sys.stderr)
            return 1

    envelopes = data["envelopes"]
    if not isinstance(envelopes, list):
        print("error: envelopes field is not a list", file=sys.stderr)
        return 1

    placeholder_session = f"official-{args.slug}"
    sanitized: list[dict[str, Any]] = []
    dropped_test_refs = 0
    for env in envelopes:
        env["session_id"] = placeholder_session
        # Strip "problem/" prefix if the model leaked the material-label
        # path into payload.file_path. The frontend file tree keys against
        # the bare filename as stored in problems.starter_files.
        fp = env.get("payload", {}).get("file_path")
        if isinstance(fp, str) and fp.startswith("problem/"):
            env["payload"]["file_path"] = fp[len("problem/"):]
            fp = env["payload"]["file_path"]
        # Defensive filter: drop any envelope that references a hidden test
        # file even though the SYSTEM prompt forbids it. test_*.py contents
        # never reach the candidate-facing replay.
        if isinstance(fp, str) and fp.startswith("test_"):
            dropped_test_refs += 1
            continue
        sanitized.append(env)
    if dropped_test_refs:
        print(f"  dropped {dropped_test_refs} envelope(s) referencing hidden test files",
              file=sys.stderr)
    envelopes = synthesize_timestamps(sanitized)

    # files: the model returns [{name, content}, ...]; collapse to {name: content}
    # so the JSONB column reads as a simple filename → content map at consume time.
    # Same prefix-stripping as for file_path - the keys must match starter_files.
    # test_* files are the arena's hidden test fixtures; they must never leak
    # into the candidate-visible final-file map. We drop them defensively here
    # even though the SYSTEM prompt also forbids exposing test internals.
    file_map: dict[str, str] = {}
    for entry in data["files"]:
        if not isinstance(entry, dict):
            continue
        name = entry.get("name")
        content = entry.get("content")
        if not (isinstance(name, str) and isinstance(content, str)):
            continue
        if name.startswith("problem/"):
            name = name[len("problem/"):]
        if name.startswith("test_"):
            print(f"  dropping hidden test file from output: {name}", file=sys.stderr)
            continue
        file_map[name] = content

    output = {
        "envelopes": envelopes,
        "files": file_map,
        "explanation_md": data["explanation_md"],
    }

    out_path = args.out or (
        Path(__file__).resolve().parent.parent / "temp" / f"{args.slug}_official_reply.json"
    )
    out_path.parent.mkdir(parents=True, exist_ok=True)
    out_path.write_text(json.dumps(output, ensure_ascii=False, indent=2), encoding="utf-8")

    surface_count = sum(1 for e in envelopes if e["kind"] in SURFACE_KINDS)
    file_edits = [
        e for e in envelopes
        if e["kind"] == "tool_use_proposed"
        and e["payload"].get("tool_name") == "FileEdit"
    ]
    with_patch = sum(1 for e in file_edits if "patch" in e["payload"])
    print(f"wrote {out_path}")
    print(f"  envelopes: {len(envelopes)} ({surface_count} surface, "
          f"{len(envelopes) - surface_count} collapse)")
    print(f"  FileEdit envelopes: {len(file_edits)} total, {with_patch} with patch")
    if len(file_edits) and with_patch < len(file_edits):
        missing = [e["seq"] for e in file_edits if "patch" not in e["payload"]]
        print(f"  WARNING: FileEdit envelopes missing patch (seq): {missing}")
    print(f"  files: {sorted(file_map.keys())}")
    print(f"  explanation_md: {len(output['explanation_md'])} chars")
    return 0


if __name__ == "__main__":
    sys.exit(main())
