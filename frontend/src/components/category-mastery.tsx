import { cn } from "@/lib/utils";
import type { Category, Submission, Problem } from "@/lib/types";

const CATEGORY_LABELS: Record<Category, string> = {
  debugging: "Debugging",
  refactoring: "Refactoring",
  feature_build: "Feature build",
  security: "Security",
  company_premium: "Company premium",
};

const CATEGORIES: Category[] = ["debugging", "refactoring", "feature_build", "security", "company_premium"];

interface CategoryMasteryProps {
  submissions: Submission[];
  problems: Problem[];
}

export function CategoryMastery({ submissions, problems }: CategoryMasteryProps) {
  const problemMap = new Map(problems.map((p) => [p.id, p]));

  const stats = CATEGORIES.map((cat) => {
    const subs = submissions.filter((s) => problemMap.get(s.problem_id)?.category === cat);
    const scored = subs.filter((s) => s.score);
    const avg =
      scored.length === 0
        ? null
        : scored.reduce((a, s) => a + (s.score?.total ?? 0), 0) / scored.length;
    return { cat, count: subs.length, avg };
  });

  return (
    <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
      {stats.map(({ cat, count, avg }) => (
        <div
          key={cat}
          className="rounded-lg border border-divider bg-surface px-3 py-3 flex items-center gap-3"
        >
          <Ring value={avg} />
          <div className="min-w-0">
            <div className="text-xs uppercase tracking-wider text-faint truncate">
              {CATEGORY_LABELS[cat]}
            </div>
            <div className="text-sm tabular-nums text-ink">
              {count === 0 ? "—" : `${count} sub${count === 1 ? "" : "s"}`}
            </div>
          </div>
        </div>
      ))}
    </div>
  );
}

function Ring({ value }: { value: number | null }) {
  const pct = value == null ? 0 : Math.max(0, Math.min(100, value));
  const r = 16;
  const c = 2 * Math.PI * r;
  const dash = (pct / 100) * c;
  const tone =
    value == null
      ? "stroke-divider"
      : value >= 80
        ? "stroke-success"
        : value >= 60
          ? "stroke-warning"
          : "stroke-danger";
  return (
    <svg width="40" height="40" viewBox="0 0 40 40" className="flex-shrink-0">
      <circle cx="20" cy="20" r={r} className="stroke-divider" strokeWidth="3" fill="none" />
      <circle
        cx="20"
        cy="20"
        r={r}
        className={cn(tone, "transition-[stroke-dashoffset]")}
        strokeWidth="3"
        fill="none"
        strokeDasharray={`${dash} ${c}`}
        strokeLinecap="round"
        transform="rotate(-90 20 20)"
      />
      <text
        x="20"
        y="22"
        textAnchor="middle"
        className="fill-ink text-[10px] font-medium tabular-nums"
      >
        {value == null ? "—" : Math.round(value)}
      </text>
    </svg>
  );
}
