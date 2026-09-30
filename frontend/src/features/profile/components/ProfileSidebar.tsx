import { MapPin, MessageSquare, MessagesSquare, ThumbsUp } from "lucide-react";
import { t, type LocaleKey } from "@/shared/i18n";
import type { ProfileData } from "@/features/profile/types";
import { cn } from "@/shared/lib/cn";
import { categoryLabel, dimensionLabel } from "@/shared/labels";
import { formatMonthYear, initialOf } from "@/shared/format";
import { EditProfileButton } from "./EditProfileButton";

// Rubric averages (1-5) bucketed like LeetCode's Advanced / Intermediate /
// Fundamental skill groups.
const STRENGTH_TIERS: { labelKey: LocaleKey; dot: string; min: number; max: number }[] = [
  { labelKey: "prof_strength_strong", dot: "bg-home-green", min: 4, max: Infinity },
  { labelKey: "prof_strength_developing", dot: "bg-home-amber", min: 3, max: 4 },
  { labelKey: "prof_strength_focus", dot: "bg-home-rose", min: -Infinity, max: 3 },
];

export function ProfileSidebar({ profile }: { profile: ProfileData }) {
  const { user, community, solved, dimensions } = profile;
  const initial = initialOf(user.display_name);
  const memberSince = formatMonthYear(user.member_since);
  const tierKey: LocaleKey = user.tier === "pro" || user.tier === "max" ? "tier_pro" : "tier_free";
  const scored = dimensions.filter((d) => d.average !== null);

  const stats = [
    { icon: MessageSquare, color: "text-home-blue", labelKey: "prof_stat_posts" as LocaleKey, value: community.posts },
    { icon: MessagesSquare, color: "text-home-teal", labelKey: "prof_stat_comments" as LocaleKey, value: community.comments },
    { icon: ThumbsUp, color: "text-home-amber", labelKey: "prof_stat_upvotes" as LocaleKey, value: community.upvotes },
  ];

  return (
    <aside className="rounded-2xl border border-divider bg-surface p-5 sm:p-6 space-y-6 self-start">
      <div className="flex gap-4">
        <span
          className="grid size-20 shrink-0 place-items-center overflow-hidden rounded-2xl text-2xl font-semibold text-white"
          style={{ background: user.avatar_color || "var(--accent)" }}
        >
          {user.avatar_url ? (
            // eslint-disable-next-line @next/next/no-img-element
            <img src={user.avatar_url} alt="" className="h-full w-full object-cover" />
          ) : (
            initial
          )}
        </span>
        <div className="min-w-0 pt-1">
          <h1 className="text-lg font-semibold truncate">{user.display_name}</h1>
          <p className="text-sm text-muted truncate">@{user.handle}</p>
          <p className="text-xs text-faint mt-2">
            {t("prof_member_since", { params: { date: memberSince } })} · {t(tierKey)}
          </p>
        </div>
      </div>

      <EditProfileButton displayName={user.display_name} bio={user.bio} region={user.region} />

      {(user.bio || user.region) && (
        <div className="space-y-2 text-sm">
          {user.bio && <p className="text-muted leading-relaxed whitespace-pre-line">{user.bio}</p>}
          {user.region && (
            <p className="flex items-center gap-2 text-muted">
              <MapPin size={15} className="text-faint" />
              {user.region}
            </p>
          )}
        </div>
      )}

      <Section title={t("prof_community_title")}>
        <ul className="space-y-3.5">
          {stats.map((s) => (
            <li key={s.labelKey} className="flex gap-3">
              <s.icon size={17} className={cn("mt-0.5 shrink-0", s.color)} />
              <div className="text-sm">
                <p>
                  {t(s.labelKey)} <span className="font-medium tabular-nums ml-1">{s.value.total}</span>
                </p>
                <p className="text-xs text-faint mt-0.5">
                  {t("prof_last_week")}{" "}
                  <span className={cn("tabular-nums", s.value.last_week > 0 && "text-home-green")}>
                    {s.value.last_week > 0 ? `+${s.value.last_week}` : 0}
                  </span>
                </p>
              </div>
            </li>
          ))}
        </ul>
      </Section>

      <Section title={t("prof_tracks_title")}>
        <ul className="space-y-3">
          {solved.by_category.map((c) => (
            <li key={c.key} className="text-sm">
              <div className="flex justify-between">
                <span>{categoryLabel(c.key)}</span>
                <span className="text-muted tabular-nums">
                  {c.solved}
                  <span className="text-faint">/{c.total}</span>
                </span>
              </div>
              <div className="h-1.5 rounded-full bg-surface-2 mt-1.5 overflow-hidden">
                <div
                  className="h-full rounded-full bg-home-blue"
                  style={{ width: `${c.total ? (c.solved / c.total) * 100 : 0}%` }}
                />
              </div>
            </li>
          ))}
        </ul>
      </Section>

      <Section title={t("prof_strengths_title")}>
        {scored.length === 0 ? (
          <p className="text-sm text-faint text-center py-2">{t("prof_not_enough_data")}</p>
        ) : (
          <div className="space-y-4">
            {STRENGTH_TIERS.map((tier) => {
              const dims = scored.filter((d) => d.average! >= tier.min && d.average! < tier.max);
              return (
                <div key={tier.labelKey}>
                  <p className="flex items-center gap-2 text-sm font-medium">
                    <span className={cn("size-1.5 rounded-full", tier.dot)} />
                    {t(tier.labelKey)}
                  </p>
                  {dims.length === 0 ? (
                    <p className="text-xs text-faint mt-1.5 pl-3.5">—</p>
                  ) : (
                    <div className="flex flex-wrap gap-1.5 mt-2 pl-3.5">
                      {dims.map((d) => (
                        <span
                          key={d.dimension}
                          className="rounded-full bg-surface-2 px-2.5 py-1 text-xs text-muted"
                          title={t("prof_strength_samples", { params: { n: d.samples } })}
                        >
                          {dimensionLabel(d.dimension)}
                          <span className="ml-1.5 text-ink tabular-nums">{d.average!.toFixed(1)}</span>
                        </span>
                      ))}
                    </div>
                  )}
                </div>
              );
            })}
          </div>
        )}
      </Section>
    </aside>
  );
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section className="pt-6 border-t border-divider">
      <h2 className="font-semibold mb-4">{title}</h2>
      {children}
    </section>
  );
}
