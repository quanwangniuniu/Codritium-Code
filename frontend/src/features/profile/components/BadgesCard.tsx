import type { CSSProperties } from "react";
import { Flame, Layers, Lock, Rocket, Send, Target, Trophy, type LucideIcon } from "lucide-react";
import { t, type LocaleKey } from "@/shared/i18n";
import type { ProfileData } from "@/features/profile/types";
import { cn } from "@/shared/lib/cn";

interface BadgeDef {
  id: string;
  icon: LucideIcon;
  accent: string;
  nameKey: LocaleKey;
  // [current, goal] progress toward the badge; earned when current >= goal.
  progress: (p: ProfileData) => [number, number];
}

// Milestone badges derived from profile data — no badge storage needed.
const BADGES: BadgeDef[] = [
  { id: "early", icon: Rocket, accent: "blue", nameKey: "prof_badge_early", progress: () => [1, 1] },
  {
    id: "first",
    icon: Send,
    accent: "teal",
    nameKey: "prof_badge_first",
    progress: (p) => [Math.min(1, p.recent_submissions.length), 1],
  },
  { id: "ten", icon: Target, accent: "green", nameKey: "prof_badge_ten", progress: (p) => [p.solved.solved, 10] },
  { id: "streak", icon: Flame, accent: "amber", nameKey: "prof_badge_streak", progress: (p) => [p.calendar.max_streak, 7] },
  {
    id: "tracks",
    icon: Layers,
    accent: "rose",
    nameKey: "prof_badge_tracks",
    progress: (p) => {
      const tracks = p.solved.by_category.filter((c) => c.total > 0);
      return [tracks.filter((c) => c.solved > 0).length, tracks.length || 1];
    },
  },
  {
    id: "ninety",
    icon: Trophy,
    accent: "amber",
    nameKey: "prof_badge_ninety",
    progress: (p) => [Math.round(p.best_score ?? 0), 90],
  },
  { id: "fifty", icon: Target, accent: "blue", nameKey: "prof_badge_fifty", progress: (p) => [p.solved.solved, 50] },
];

export function BadgesCard({ profile }: { profile: ProfileData }) {
  const rows = BADGES.map((b) => {
    const [cur, goal] = b.progress(profile);
    return { ...b, cur: Math.min(cur, goal), goal, earned: cur >= goal };
  });
  const earned = rows.filter((r) => r.earned);
  const next = rows.find((r) => !r.earned);

  return (
    <div className="rounded-2xl border border-divider bg-surface p-5 sm:p-6 flex flex-col">
      <p className="text-sm text-muted">{t("prof_badges_title")}</p>
      <p className="text-3xl font-semibold tracking-tight mt-1">{earned.length}</p>

      <div className="flex flex-wrap gap-2 mt-4">
        {rows.map((b) => (
          <span
            key={b.id}
            title={t(b.nameKey)}
            className={cn("home-badge !size-10 !rounded-xl", !b.earned && "opacity-25 grayscale")}
            style={{ "--badge": `var(--home-${b.accent})` } as CSSProperties}
          >
            <b.icon size={18} />
            <span className="sr-only">
              {t(b.nameKey)} {b.earned ? "" : t("prof_badge_locked")}
            </span>
          </span>
        ))}
      </div>

      {next && (
        <div className="mt-auto pt-5">
          <p className="flex items-center gap-1.5 text-xs text-muted">
            <Lock size={12} /> {t("prof_badge_next")}
          </p>
          <div className="flex items-baseline justify-between gap-3 mt-1">
            <p className="font-medium">{t(next.nameKey)}</p>
            <p className="text-xs text-muted tabular-nums">
              {next.cur}/{next.goal}
            </p>
          </div>
          <div className="h-1.5 rounded-full bg-surface-2 mt-2 overflow-hidden">
            <div
              className="h-full rounded-full bg-accent"
              style={{ width: `${(next.cur / next.goal) * 100}%` }}
            />
          </div>
        </div>
      )}
    </div>
  );
}
