"use client";

import { useEffect, useRef, useState } from "react";
import { cn } from "@/shared/lib/cn";

// Section glyphs for the home page, drawn in the logo's language: square
// cells on a grid. Each pattern is 9x9; one character per cell:
//   #  ink cell          +  accent cell
//   o  faint ink cell    .  empty
// Keeping them as text makes a glyph reviewable (and editable) at a glance.
const GLYPHS = {
  // "Real problems, not puzzles": a bug.
  problems: [
    ".#.....#.",
    "..#...#..",
    "...###...",
    "#.#####.#",
    ".##+#+##.",
    "..#####..",
    ".#######.",
    "#..###..#",
    ".........",
  ],
  // "An AI agent in your editor": code with a removed line and an added block.
  agent: [
    "#####....",
    ".........",
    "..ooooo..",
    ".........",
    "..++++++.",
    "..++++...",
    ".........",
    "####.....",
    ".........",
  ],
  // "Scored on five dimensions": the five rubric bars.
  scoring: [
    "#######oo",
    ".........",
    "#####oooo",
    ".........",
    "++++++++o",
    ".........",
    "######ooo",
    ".........",
    "#######oo",
  ],
  // "Problems & community": two speech bubbles.
  community: [
    "######...",
    "#....#...",
    "#....#...",
    "######...",
    ".#.++++++",
    "...+....+",
    "...+....+",
    "...++++++",
    ".......+.",
  ],
  // "For hiring teams": an assessment sheet with a tick.
  hiring: [
    "..#####..",
    ".##...##.",
    ".#.....#.",
    ".#....+#.",
    ".#...+.#.",
    ".#+.+..#.",
    ".#.+...#.",
    ".#.....#.",
    ".#######.",
  ],
  // "Try a real problem": a shell prompt and cursor.
  showcase: [
    ".........",
    ".#.......",
    "..#......",
    "...#.....",
    "....#....",
    "...#.....",
    "..#......",
    ".#..++++.",
    ".........",
  ],
  // "Built for the AI-enabled interview": you and the agent, overlapping.
  mission: [
    "######...",
    "######...",
    "######...",
    "###+++ooo",
    "###+++ooo",
    "###+++ooo",
    "...oooooo",
    "...oooooo",
    "...oooooo",
  ],
} satisfies Record<string, string[]>;

export type GlyphName = keyof typeof GLYPHS;

const GRID = 9;
// Cell edge within its 1x1 slot; the remainder is the gap between cells,
// matching the tight spacing of the logo's block.
const CELL = 0.84;

const CELL_CLASS: Record<string, string> = {
  "#": "fill-ink",
  "+": "fill-accent",
  o: "fill-ink opacity-25",
};

// Deterministic per-cell scatter offset (in cells), so the settle animation
// is the same on server and client and never causes a hydration mismatch.
function scatter(col: number, row: number): [number, number] {
  const n = Math.sin(col * 12.9898 + row * 78.233) * 43758.5453;
  const f = n - Math.floor(n);
  return [(f - 0.5) * 3, (((f * 7) % 1) - 0.5) * 3];
}

interface DotGlyphProps {
  name: GlyphName;
  size?: number;
  className?: string;
}

// One glyph. Cells drift in from a small scatter and settle into the grid
// the first time the glyph scrolls into view (the logo's scatter-to-block
// idea); with reduced motion, or before hydration, they are simply in place.
export function DotGlyph({ name, size = 56, className }: DotGlyphProps) {
  const ref = useRef<SVGSVGElement>(null);
  // "idle": rendered in place (SSR, no JS, reduced motion).
  // "scattered": armed off-screen, waiting to be seen. "settled": animated in.
  const [phase, setPhase] = useState<"idle" | "scattered" | "settled">("idle");

  useEffect(() => {
    const el = ref.current;
    if (!el || window.matchMedia("(prefers-reduced-motion: reduce)").matches) return;
    // Already on screen at load: leave it alone rather than replaying.
    if (el.getBoundingClientRect().top < window.innerHeight) return;
    setPhase("scattered");
    const observer = new IntersectionObserver(
      (entries) => {
        if (entries.some((e) => e.isIntersecting)) {
          setPhase("settled");
          observer.disconnect();
        }
      },
      { threshold: 0.6 },
    );
    observer.observe(el);
    return () => observer.disconnect();
  }, []);

  const cells = GLYPHS[name].flatMap((line, row) =>
    [...line].map((ch, col) => ({ ch, col, row })).filter((c) => c.ch !== "."),
  );

  return (
    <svg
      ref={ref}
      aria-hidden
      width={size}
      height={size}
      viewBox={`0 0 ${GRID} ${GRID}`}
      className={cn("block overflow-visible", className)}
    >
      {cells.map(({ ch, col, row }, i) => {
        const [dx, dy] = scatter(col, row);
        return (
          <rect
            key={`${col}-${row}`}
            x={col + (1 - CELL) / 2}
            y={row + (1 - CELL) / 2}
            width={CELL}
            height={CELL}
            className={CELL_CLASS[ch]}
            style={
              phase === "idle"
                ? undefined
                : {
                    transitionProperty: "transform, opacity",
                    transitionDuration: "600ms, 400ms",
                    transitionTimingFunction: "cubic-bezier(0.2, 0.8, 0.2, 1)",
                    // Stagger only the settle; arming must be instant.
                    transitionDelay: phase === "settled" ? `${(i % 12) * 18}ms` : "0ms",
                    transform: phase === "scattered" ? `translate(${dx}px, ${dy}px)` : undefined,
                    opacity: phase === "scattered" ? 0 : undefined,
                  }
            }
          />
        );
      })}
    </svg>
  );
}
