import { cn } from "@/lib/utils";

interface CalendarHeatmapProps {
  submissions: { submitted_at: string; score: { total: number } | null }[];
  weeks?: number;
}

const MS_DAY = 86_400_000;

function dayKey(d: Date) {
  return `${d.getUTCFullYear()}-${String(d.getUTCMonth() + 1).padStart(2, "0")}-${String(d.getUTCDate()).padStart(2, "0")}`;
}

// Intensity is driven by daily submission count — 10/day saturates the
// scale, matching the typical "lots of practice" feel without rewarding
// runaway grinding. Tier boundaries: 1-2 / 3-5 / 6-9 / 10+.
function intensity(count: number): 0 | 1 | 2 | 3 | 4 {
  if (count <= 0) return 0;
  if (count <= 2) return 1;
  if (count <= 5) return 2;
  if (count <= 9) return 3;
  return 4;
}

const TONES: Record<0 | 1 | 2 | 3 | 4, string> = {
  0: "bg-surface-2",
  1: "bg-accent/20",
  2: "bg-accent/40",
  3: "bg-accent/65",
  4: "bg-accent",
};

export function CalendarHeatmap({ submissions, weeks = 12 }: CalendarHeatmapProps) {
  const today = new Date();
  today.setUTCHours(0, 0, 0, 0);
  const todayDow = today.getUTCDay();
  const gridEnd = new Date(today.getTime() + (6 - todayDow) * MS_DAY);
  const gridStart = new Date(gridEnd.getTime() - (weeks * 7 - 1) * MS_DAY);

  const counts = new Map<string, number>();
  for (const s of submissions) {
    const k = dayKey(new Date(s.submitted_at));
    counts.set(k, (counts.get(k) ?? 0) + 1);
  }

  const cols: { date: Date; level: 0 | 1 | 2 | 3 | 4; count: number; future: boolean }[][] = [];
  for (let w = 0; w < weeks; w++) {
    const col: typeof cols[number] = [];
    for (let d = 0; d < 7; d++) {
      const date = new Date(gridStart.getTime() + (w * 7 + d) * MS_DAY);
      const future = date.getTime() > today.getTime();
      const k = dayKey(date);
      const count = counts.get(k) ?? 0;
      col.push({ date, level: future ? 0 : intensity(count), count, future });
    }
    cols.push(col);
  }

  const monthLabels: { col: number; label: string }[] = [];
  let lastMonth = -1;
  cols.forEach((col, i) => {
    const m = col[0].date.getUTCMonth();
    if (m !== lastMonth) {
      monthLabels.push({ col: i, label: col[0].date.toLocaleString("en", { month: "short" }) });
      lastMonth = m;
    }
  });

  const totalDays = cols.flat().filter((c) => c.count > 0 && !c.future).length;

  return (
    <div className="space-y-2">
      <div className="flex items-end gap-2">
        <div className="flex flex-col gap-[3px] pt-[14px] text-[10px] text-faint pr-1 leading-none">
          <span>Mon</span>
          <span>&nbsp;</span>
          <span>Wed</span>
          <span>&nbsp;</span>
          <span>Fri</span>
          <span>&nbsp;</span>
          <span>Sun</span>
        </div>
        <div className="flex-1">
          <div className="relative h-3 mb-1 text-[10px] text-faint">
            {monthLabels.map((m) => (
              <span
                key={m.col}
                className="absolute leading-none"
                style={{ left: `calc(${(m.col / weeks) * 100}% )` }}
              >
                {m.label}
              </span>
            ))}
          </div>
          <div className="flex gap-[3px]">
            {cols.map((col, i) => (
              <div key={i} className="flex flex-col gap-[3px]">
                {col.map((cell, j) => (
                  <div
                    key={j}
                    title={cell.future ? "" : `${dayKey(cell.date)} · ${cell.count} submission(s)`}
                    className={cn(
                      "h-3 w-3 rounded-[3px]",
                      cell.future ? "bg-transparent" : TONES[cell.level],
                    )}
                  />
                ))}
              </div>
            ))}
          </div>
        </div>
      </div>
      <div className="flex items-center justify-between text-[11px] text-muted">
        <span>{totalDays} active day{totalDays === 1 ? "" : "s"} in the last {weeks} weeks</span>
        <div className="flex items-center gap-1.5">
          <span className="text-faint">Less</span>
          {[0, 1, 2, 3, 4].map((l) => (
            <span
              key={l}
              className={cn("h-2.5 w-2.5 rounded-[2px]", TONES[l as 0 | 1 | 2 | 3 | 4])}
            />
          ))}
          <span className="text-faint">More</span>
        </div>
      </div>
    </div>
  );
}
