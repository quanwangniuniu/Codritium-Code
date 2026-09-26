#!/usr/bin/env python3
"""Strip an Arena problem source directory into Codritium-ready JSON.

Produces a JSON file with:
  - slug, title, category, difficulty, readme_md
  - starter_files: dict keyed by filename, each value containing both 'as-is' and 'stripped' variants
  - hidden_test_file: filename of the grader oracle (kept separate, NOT in starter_files)

Stripping rules (per D_problem_strip_rules.md):
  - 'as-is': keep file verbatim including '# Bug:' / '# TODO:' single-line comments
  - 'stripped': remove '# Bug:' / '# TODO:' single-line comments, and remove block
    comments containing the keywords 'broken' / 'fix this' / 'anti-pattern'

The hidden grader file (default: test_answer.py) is captured separately and
NEVER appears in starter_files (per OQ5: hidden test must not enter candidate
sandbox).

Usage:
  python scripts/strip_problem.py <arena_problem_dir> [--out seed/problems/<slug>.json]
"""

from __future__ import annotations

import argparse
import json
import os
import re
import sys
from pathlib import Path


HIDDEN_TEST_FILENAME = "test_answer.py"
SINGLE_LINE_STRIP_PREFIXES = ("# Bug:", "# TODO:", "# FIXME:")
BROKEN_KEYWORDS = ("broken", "fix this", "anti-pattern", "anti pattern")


def strip_source(content: str) -> str:
    out_lines: list[str] = []
    for line in content.splitlines(keepends=False):
        stripped = line.lstrip()
        if any(stripped.startswith(p) for p in SINGLE_LINE_STRIP_PREFIXES):
            continue
        out_lines.append(line)
    text = "\n".join(out_lines)
    text = _strip_block_comments_with_keywords(text)
    if content.endswith("\n"):
        text += "\n"
    return text


def _strip_block_comments_with_keywords(text: str) -> str:
    # Match triple-quoted strings (docstrings/block comments) containing broken keywords.
    pattern = re.compile(r'(""".*?""")', re.DOTALL)

    def keep_or_drop(m: re.Match) -> str:
        block = m.group(1)
        low = block.lower()
        if any(kw in low for kw in BROKEN_KEYWORDS):
            return ""
        return block

    return pattern.sub(keep_or_drop, text)


def parse_readme(content: str) -> dict:
    title = ""
    category = ""
    difficulty = ""
    for line in content.splitlines()[:20]:
        if line.startswith("# ") and not title:
            title = line[2:].strip()
        m = re.match(r"\*\*Category:\*\*\s*(.+)$", line)
        if m:
            raw = m.group(1).strip().lower()
            # "Feature Build" -> "feature_build"; "Refactoring" -> "refactoring"
            category = raw.replace("-", "_").replace(" ", "_")
        m = re.match(r"\*\*Difficulty:\*\*\s*(\S+)", line)
        if m:
            difficulty = m.group(1).strip().lower()
    return {"title": title, "category": category, "difficulty": difficulty}


def main() -> int:
    p = argparse.ArgumentParser()
    p.add_argument("source", type=Path, help="Arena problem directory")
    p.add_argument("--out", type=Path, default=None, help="Output JSON path")
    p.add_argument("--slug", type=str, default=None, help="Override slug (defaults to dir name)")
    args = p.parse_args()

    src: Path = args.source.resolve()
    if not src.is_dir():
        print(f"error: not a directory: {src}", file=sys.stderr)
        return 1

    readme_path = src / "README.md"
    if not readme_path.exists():
        print(f"error: missing README.md in {src}", file=sys.stderr)
        return 1

    slug = args.slug or src.name
    readme_md = readme_path.read_text(encoding="utf-8")
    readme_meta = parse_readme(readme_md)

    starter_files: dict[str, dict[str, str]] = {}
    hidden_test_content = ""

    for child in sorted(src.iterdir()):
        if child.is_dir():
            continue
        name = child.name
        if name == "README.md":
            continue
        if name.startswith(".") or name.endswith(".pyc"):
            continue
        if name == "__pycache__":
            continue

        content = child.read_text(encoding="utf-8")

        if name == HIDDEN_TEST_FILENAME:
            hidden_test_content = content
            continue

        starter_files[name] = {
            "as-is": content,
            "stripped": strip_source(content),
        }

    if not hidden_test_content:
        print(f"warning: no {HIDDEN_TEST_FILENAME} found in {src}", file=sys.stderr)

    out_path = args.out or Path(f"seed/problems/{slug}.json")
    out_path.parent.mkdir(parents=True, exist_ok=True)

    record = {
        "slug": slug,
        "title": readme_meta["title"],
        "category": readme_meta["category"],
        "difficulty": readme_meta["difficulty"],
        "readme_md": readme_md,
        "starter_files": starter_files,
        "hidden_test_filename": HIDDEN_TEST_FILENAME,
        "hidden_test_content": hidden_test_content,
        "strip_variant": "both",
    }

    out_path.write_text(json.dumps(record, ensure_ascii=False, indent=2), encoding="utf-8")
    print(f"wrote {out_path}")
    print(f"  title={readme_meta['title']!r}")
    print(f"  category={readme_meta['category']} difficulty={readme_meta['difficulty']}")
    print(f"  starter_files={list(starter_files.keys())}")
    print(f"  hidden_test={HIDDEN_TEST_FILENAME!r} ({len(hidden_test_content)} bytes)")
    return 0


if __name__ == "__main__":
    sys.exit(main())
