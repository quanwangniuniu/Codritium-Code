#!/usr/bin/env python3
"""Import dogfood arena problems into Codritium seed JSON.

For each slug, locates `arenaresearch/<category>/<difficulty>/<slug>/`,
reads `problem/` for starter files + hidden test + README, reads
`research/phase-C/05_soul_md_prebake.md` for tips-agent guidance, and
writes `seed/problems/<slug>.json` with the same schema strip_problem.py
emits plus a new `soul_prebake` field.

The hidden test file (`test_answer.py`) is captured into
`hidden_test_content` and NEVER appears in `starter_files`, matching the
existing strip rules.

Usage:
  python scripts/import_arenaresearch.py
  python scripts/import_arenaresearch.py --arena ../arenaresearch --out seed/problems
  python scripts/import_arenaresearch.py --slugs 01-broken-discount-calculator,04-order-state-machine
"""

from __future__ import annotations

import argparse
import json
import re
import sys
from pathlib import Path


HIDDEN_TEST_FILENAME = "test_answer.py"
SINGLE_LINE_STRIP_PREFIXES = ("# Bug:", "# TODO:", "# FIXME:")
BROKEN_KEYWORDS = ("broken", "fix this", "anti-pattern", "anti pattern")

DOGFOOD_SLUGS = (
    "01-broken-discount-calculator",
    "04-order-state-machine",
    "07-inventory-oversell",
    "29-fix-stored-xss",
)


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
            category = raw.replace("-", "_").replace(" ", "_")
        m = re.match(r"\*\*Difficulty:\*\*\s*(\S+)", line)
        if m:
            difficulty = m.group(1).strip().lower()
    return {"title": title, "category": category, "difficulty": difficulty}


def locate_problem(arena: Path, slug: str) -> Path:
    matches = [p for p in arena.rglob(slug) if p.is_dir()]
    if not matches:
        raise FileNotFoundError(f"no arena directory matches slug {slug!r} under {arena}")
    if len(matches) > 1:
        raise RuntimeError(f"slug {slug!r} matched multiple arena directories: {matches}")
    return matches[0]


def build_record(arena_problem_dir: Path, slug: str) -> dict:
    readme_path = arena_problem_dir / "problem" / "README.md"
    if not readme_path.exists():
        raise FileNotFoundError(f"missing README.md at {readme_path}")
    readme_md = readme_path.read_text(encoding="utf-8")
    meta = parse_readme(readme_md)

    starter_files: dict[str, dict[str, str]] = {}
    hidden_test_content = ""

    problem_dir = arena_problem_dir / "problem"
    for child in sorted(problem_dir.iterdir()):
        if child.is_dir():
            continue
        name = child.name
        if name == "README.md" or name.startswith(".") or name.endswith(".pyc"):
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
        raise FileNotFoundError(f"missing {HIDDEN_TEST_FILENAME} in {problem_dir}")

    prebake_path = arena_problem_dir / "research" / "phase-C" / "05_soul_md_prebake.md"
    if not prebake_path.exists():
        raise FileNotFoundError(f"missing prebake at {prebake_path}")
    soul_prebake = prebake_path.read_text(encoding="utf-8")

    return {
        "slug": slug,
        "title": meta["title"],
        "category": meta["category"],
        "difficulty": meta["difficulty"],
        "readme_md": readme_md,
        "starter_files": starter_files,
        "hidden_test_filename": HIDDEN_TEST_FILENAME,
        "hidden_test_content": hidden_test_content,
        "strip_variant": "both",
        "soul_prebake": soul_prebake,
    }


def main() -> int:
    p = argparse.ArgumentParser(description="Import dogfood arena problems into seed JSON.")
    p.add_argument(
        "--arena",
        type=Path,
        default=Path(__file__).resolve().parent.parent.parent / "arenaresearch",
        help="Path to arenaresearch root (default: ../arenaresearch relative to repo)",
    )
    p.add_argument(
        "--out",
        type=Path,
        default=Path(__file__).resolve().parent.parent / "seed" / "problems",
        help="Output directory for seed JSON files",
    )
    p.add_argument(
        "--slugs",
        type=str,
        default=",".join(DOGFOOD_SLUGS),
        help="Comma-separated slug list (default: 4 dogfood slugs)",
    )
    args = p.parse_args()

    arena: Path = args.arena.resolve()
    if not arena.is_dir():
        print(f"error: arena directory not found: {arena}", file=sys.stderr)
        return 1

    args.out.mkdir(parents=True, exist_ok=True)

    slugs = [s.strip() for s in args.slugs.split(",") if s.strip()]
    if not slugs:
        print("error: empty slug list", file=sys.stderr)
        return 1

    for slug in slugs:
        try:
            problem_dir = locate_problem(arena, slug)
        except (FileNotFoundError, RuntimeError) as exc:
            print(f"error: {exc}", file=sys.stderr)
            return 1

        record = build_record(problem_dir, slug)
        out_path = args.out / f"{slug}.json"
        out_path.write_text(json.dumps(record, ensure_ascii=False, indent=2), encoding="utf-8")
        print(f"wrote {out_path}")
        print(f"  title={record['title']!r} category={record['category']} difficulty={record['difficulty']}")
        print(f"  starter_files={list(record['starter_files'].keys())}")
        print(f"  hidden_test={record['hidden_test_filename']!r} ({len(record['hidden_test_content'])} bytes)")
        print(f"  soul_prebake={len(record['soul_prebake'])} bytes")

    return 0


if __name__ == "__main__":
    sys.exit(main())
