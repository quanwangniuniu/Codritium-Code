import { t, type LocaleKey } from "@/lib/i18n";
import type { ProfileData } from "@/lib/types";
import { cn } from "@/lib/utils";

const DIFFICULTY: Record<string, { labelKey: LocaleKey; text: string; stroke: string }> = {
  easy: { labelKey: "problems_difficulty_easy", text: "text-home-teal", stroke: "var(--home-teal)" },
  medium: { labelKey: "problems_difficulty_medium", text: "text-home-amber", stroke: "var(--home-amber)" },
  hard: { labelKey: "problems_difficulty_hard", text: "text-home-rose", stroke: "var(--home-rose)" },
};

// The ring spans 290° with the gap at the bottom; each difficulty gets an
// arc sized by its share of the catalog, filled by its share solved.
const SWEEP = 290;
const START = 90 + (360 - SWEEP) / 2; // degrees, 0 = 3 o'clock, clockwise
const GAP = 5;

function polar(cx: number, cy: number, r: number, deg: number) {
  const rad = (deg * Math.PI) / 180;
  return { x: cx + r * Math.cos(rad), y: cy + r * Math.sin(rad) };
}

function arc(cx: number, cy: number, r: number, from: number, to: number) {
  const a = polar(cx, cy, r, from);
  const b = polar(cx, cy, r, to);
  const large = to - from > 180 ? 1 : 0;
  return `M ${a.x} ${a.y} A ${r} ${r} 0 ${large} 1 ${b.x} ${b.y}`;
}

export function SolvedCard({ solved }: { solved: ProfileData["solved"] }) {
  const buckets = solved.by_difficulty.filter((b) => DIFFICULTY[b.key]);
  const total = buckets.reduce((n, b) => n + b.total, 0) || 1;
  const usable = SWEEP - GAP * Math.max(0, buckets.length - 1);

  let cursor = START;
  const segments = buckets.map((b) => {
    const span = (b.total / total) * usable;
    const seg = {
      key: b.key,
      from: cursor,
      to: cursor + span,
      filled: cursor + (b.total ? (b.solved / b.total) * span : 0),
    };
    cursor += span + GAP;
    return seg;
  });

  const size = 176;
  const c = size / 2;
  const r = c - 10;

  return (
    <div className="rounded-2xl border border-divider bg-surface p-5 sm:p-6 flex flex-col sm:flex-row items-center gap-6">
      <div className="relative shrink-0" style={{ width: size, height: size }}>
        <svg width={size} height={size} viewBox={`0 0 ${size} ${size}`} aria-hidden>
          {segments.map((s) => (
            <g key={s.key}>
              <path
                d={arc(c, c, r, s.from, s.to)}
                fill="none"
                stroke={DIFFICULTY[s.key].stroke}
                strokeOpacity={0.18}
                strokeWidth={7}
                strokeLinecap="round"
              />
              {s.filled > s.from + 0.5 && (
                <path
                  d={arc(c, c, r, s.from, s.filled)}
                  fill="none"
                  stroke={DIFFICULTY[s.key].stroke}
                  strokeWidth={7}
                  strokeLinecap="round"
                />
              )}
            </g>
          ))}
        </svg>
        <div className="absolute inset-0 flex flex-col items-center justify-center text-center">
          <p className="leading-none">
            <span className="text-4xl font-semibold tracking-tight">{solved.solved}</span>
            <span className="text-sm text-muted">/{solved.total}</span>
          </p>
          <p className="mt-1.5 text-sm text-ink">{t("prof_solved_label")}</p>
          <p className="absolute bottom-1 text-xs text-muted">
            {t("prof_attempting", { params: { n: solved.attempting } })}
          </p>
        </div>
      </div>

      <div className="grid grid-cols-3 sm:grid-cols-1 gap-2.5 w-full sm:w-auto sm:min-w-[132px] sm:ml-auto">
        {buckets.map((b) => (
          <div key={b.key} className="rounded-xl bg-surface-2 px-4 py-2.5 text-center">
            <p className={cn("text-sm font-medium", DIFFICULTY[b.key].text)}>{t(DIFFICULTY[b.key].labelKey)}</p>
            <p className="text-sm tabular-nums mt-0.5">
              {b.solved}
              <span className="text-muted">/{b.total}</span>
            </p>
          </div>
        ))}
      </div>
    </div>
  );
}
