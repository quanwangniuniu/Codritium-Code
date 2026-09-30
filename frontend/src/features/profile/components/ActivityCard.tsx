import { t } from "@/shared/i18n";
import type { ProfileData } from "@/features/profile/types";
import { cn } from "@/shared/lib/cn";

const MS_DAY = 86_400_000;
const WEEKS = 53;

function dayKey(d: Date) {
  return d.toISOString().slice(0, 10);
}

// 0 = none, then 1 / 2 / 3-4 / 5+ submissions a day.
function level(count: number) {
  if (count <= 0) return 0;
  if (count === 1) return 1;
  if (count === 2) return 2;
  if (count <= 4) return 3;
  return 4;
}

const TONE = ["bg-surface-2", "bg-home-green/30", "bg-home-green/55", "bg-home-green/80", "bg-home-green"];

// A year of daily submission counts (UTC days, matching the API), laid out
// as week columns with month labels, like LeetCode's submission calendar.
export function ActivityCard({ calendar }: { calendar: ProfileData["calendar"] }) {
  const counts = new Map(calendar.days.map((d) => [d.date, d.count]));
  const today = new Date();
  today.setUTCHours(0, 0, 0, 0);
  const gridEnd = new Date(today.getTime() + (6 - today.getUTCDay()) * MS_DAY);
  const gridStart = new Date(gridEnd.getTime() - (WEEKS * 7 - 1) * MS_DAY);

  const weeks = Array.from({ length: WEEKS }, (_, w) =>
    Array.from({ length: 7 }, (_, d) => {
      const date = new Date(gridStart.getTime() + (w * 7 + d) * MS_DAY);
      const key = dayKey(date);
      return { key, count: counts.get(key) ?? 0, future: date > today };
    }),
  );

  const months: { col: number; label: string }[] = [];
  weeks.forEach((week, i) => {
    // Label the week column that contains the 1st of a month.
    const first = week.find((d) => d.key.endsWith("-01"));
    if (first && i < WEEKS - 1) {
      const label = new Date(first.key).toLocaleString("en", { month: "short", timeZone: "UTC" });
      months.push({ col: i, label });
    }
  });

  return (
    <div className="rounded-2xl border border-divider bg-surface p-5 sm:p-6">
      <div className="flex flex-wrap items-baseline justify-between gap-x-6 gap-y-2">
        <p className="text-muted">
          <span className="text-xl font-semibold text-ink tabular-nums mr-1.5">{calendar.total_submissions}</span>
          {t("prof_activity_title")}
        </p>
        <div className="flex flex-wrap gap-x-5 gap-y-1 text-sm text-muted">
          <span>
            {t("prof_active_days")} <b className="text-ink tabular-nums">{calendar.active_days}</b>
          </span>
          <span>
            {t("prof_current_streak")} <b className="text-ink tabular-nums">{calendar.current_streak}</b>
          </span>
          <span>
            {t("prof_max_streak")} <b className="text-ink tabular-nums">{calendar.max_streak}</b>
          </span>
        </div>
      </div>

      <div className="mt-5 overflow-x-auto pb-1 [direction:rtl]">
        <div className="inline-block min-w-full [direction:ltr]">
          <div className="flex gap-[3px] justify-between">
            {weeks.map((week, i) => (
              <div key={i} className="flex flex-col gap-[3px]">
                {week.map((d) => (
                  <span
                    key={d.key}
                    title={d.future ? undefined : t("prof_day_tooltip", { params: { n: d.count, date: d.key } })}
                    className={cn(
                      "size-[11px] rounded-[3px]",
                      d.future ? "bg-transparent" : TONE[level(d.count)],
                    )}
                  />
                ))}
              </div>
            ))}
          </div>
          <div className="relative h-5 mt-2 text-xs text-faint">
            {months.map((m) => (
              <span key={m.col} className="absolute" style={{ left: `${(m.col / WEEKS) * 100}%` }}>
                {m.label}
              </span>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}
