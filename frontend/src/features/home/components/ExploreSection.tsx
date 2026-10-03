import type { ReactNode } from "react";
import { Check, GitPullRequestArrow, ShieldCheck } from "lucide-react";
import { t, type LocaleKey } from "@/shared/i18n";
import { cn } from "@/shared/lib/cn";
import { ArrowLink } from "./ArrowLink";
import { DotGlyph, type GlyphName } from "./DotGlyph";

// Colour of a category's bar in the catalog visual.
export type HomeAccent = "blue" | "green" | "amber" | "teal" | "rose";

export interface CategoryStat {
  labelKey: LocaleKey;
  count: number;
  accent: HomeAccent;
}

export interface DifficultyStat {
  labelKey: LocaleKey;
  count: number;
}

interface ExploreSectionProps {
  categories: CategoryStat[];
  difficulties: DifficultyStat[];
}

export function ExploreSection({ categories, difficulties }: ExploreSectionProps) {
  return (
    <section className="mx-auto max-w-[1200px] px-4 sm:px-6 pt-10 pb-20 lg:pt-12 lg:pb-24">
      <div className="text-center max-w-2xl mx-auto mb-16 lg:mb-20">
        <h2 className="text-3xl sm:text-4xl font-semibold tracking-tight">{t("home_explore_title")}</h2>
        <p className="mt-4 text-muted leading-relaxed">{t("home_explore_blurb")}</p>
      </div>
      <div className="space-y-20 lg:space-y-28">
        <FeatureRow
          glyph="problems"
          title={t("home_feature_problems_title")}
          desc={t("home_feature_problems_desc")}
          href="/problems"
          linkLabel={t("home_feature_problems_link")}
          visual={<CategoriesVisual categories={categories} difficulties={difficulties} />}
        />
        <FeatureRow
          reverse
          glyph="agent"
          title={t("home_feature_agent_title")}
          desc={t("home_feature_agent_desc")}
          href="/problems"
          linkLabel={t("home_feature_agent_link")}
          visual={<PatchVisual />}
        />
        <FeatureRow
          glyph="scoring"
          title={t("home_feature_scoring_title")}
          desc={t("home_feature_scoring_desc")}
          href="/problems"
          linkLabel={t("home_feature_scoring_link")}
          visual={<ScoringVisual />}
        />
      </div>
    </section>
  );
}

interface FeatureRowProps {
  glyph: GlyphName;
  title: string;
  desc: string;
  href: string;
  linkLabel: string;
  visual: ReactNode;
  reverse?: boolean;
}

function FeatureRow({ glyph, title, desc, href, linkLabel, visual, reverse }: FeatureRowProps) {
  return (
    <div className="grid lg:grid-cols-2 gap-10 lg:gap-20 items-center">
      <div className={cn("space-y-4", reverse && "lg:order-2")}>
        <DotGlyph name={glyph} />
        <h3 className="text-2xl sm:text-[1.75rem] font-semibold tracking-tight">{title}</h3>
        <p className="text-muted leading-relaxed max-w-lg">{desc}</p>
        <ArrowLink href={href}>{linkLabel}</ArrowLink>
      </div>
      <div className={cn(reverse && "lg:order-1")} aria-hidden>
        {visual}
      </div>
    </div>
  );
}

const BAR_BG: Record<HomeAccent, string> = {
  blue: "bg-home-blue",
  green: "bg-home-green",
  amber: "bg-home-amber",
  teal: "bg-home-teal",
  rose: "bg-home-rose",
};

function CategoriesVisual({ categories, difficulties }: ExploreSectionProps) {
  const max = Math.max(1, ...categories.map((c) => c.count));
  const total = difficulties.reduce((n, d) => n + d.count, 0) || 1;
  const diffColor = ["bg-home-green", "bg-home-amber", "bg-home-rose"];
  return (
    <div className="home-window p-6 space-y-6">
      <div className="space-y-4">
        {categories.map((c) => (
          <div key={c.labelKey}>
            <div className="flex justify-between text-sm mb-1.5">
              <span className="font-medium">{t(c.labelKey)}</span>
              <span className="text-muted">{t("home_visual_problem_count", { params: { n: c.count } })}</span>
            </div>
            <div className="h-2.5 rounded-full bg-surface-2 overflow-hidden">
              <div
                className={cn("h-full rounded-full", BAR_BG[c.accent])}
                style={{ width: `${Math.max(6, (c.count / max) * 100)}%` }}
              />
            </div>
          </div>
        ))}
      </div>
      <div className="pt-5 border-t border-divider">
        <div className="flex h-2 rounded-full overflow-hidden gap-0.5">
          {difficulties.map((d, i) => (
            <div key={d.labelKey} className={diffColor[i]} style={{ width: `${(d.count / total) * 100}%` }} />
          ))}
        </div>
        <div className="flex gap-5 mt-3 text-xs text-muted">
          {difficulties.map((d, i) => (
            <span key={d.labelKey} className="inline-flex items-center gap-1.5">
              <span className={cn("size-2 rounded-full", diffColor[i])} />
              {t(d.labelKey)} · {d.count}
            </span>
          ))}
        </div>
      </div>
    </div>
  );
}

function PatchVisual() {
  return (
    <div className="home-window overflow-hidden">
      <div className="flex items-center gap-2 px-4 h-11 border-b border-divider text-sm">
        <GitPullRequestArrow size={15} className="text-home-teal" />
        <span className="font-medium">{t("home_visual_patch_title")}</span>
        <span className="ml-auto text-xs text-muted mono">rate_limiter.py</span>
      </div>
      <pre className="mono text-[12px] leading-[1.9] py-3">
        <div className="px-4 text-faint">@@ def allow(self, key):</div>
        <div className="px-4 bg-home-rose-soft">-    bucket = self.buckets[key]</div>
        <div className="px-4 bg-home-green-soft">+    bucket = self.buckets.setdefault(key, Bucket())</div>
        <div className="px-4 bg-home-green-soft">+    bucket.refill(self.clock())</div>
        <div className="px-4">     return bucket.take()</div>
      </pre>
      <div className="px-4 py-3 border-t border-divider bg-canvas/60 flex flex-wrap items-center gap-2 text-xs">
        <span className="text-muted mr-auto">{t("home_visual_patch_hint")}</span>
        <span className="rounded-md border border-divider px-2.5 py-1 text-muted">{t("home_visual_patch_edit")}</span>
        <span className="rounded-md border border-divider px-2.5 py-1 text-muted">{t("home_mock_reject")}</span>
        <span className="rounded-md bg-home-green text-white px-2.5 py-1 inline-flex items-center gap-1">
          <Check size={12} /> {t("home_mock_approve")}
        </span>
      </div>
    </div>
  );
}

const DIMENSIONS: { key: LocaleKey; value: number }[] = [
  { key: "dim_label_correctness", value: 4.5 },
  { key: "dim_label_decomposition", value: 4 },
  { key: "dim_label_ai_collab", value: 4.5 },
  { key: "dim_label_verification", value: 3.5 },
  { key: "dim_label_communication", value: 4 },
];

const ANTI_PATTERNS: LocaleKey[] = [
  "antipattern_hands_off_name",
  "antipattern_feature_marathon_name",
  "antipattern_ai_showcase_name",
  "antipattern_not_thinking_name",
];

function ScoringVisual() {
  return (
    <div className="home-window p-6">
      <div className="space-y-3.5">
        {DIMENSIONS.map((d) => (
          <div key={d.key} className="grid grid-cols-[150px_1fr_32px] items-center gap-3 text-sm">
            <span className="text-muted truncate">{t(d.key)}</span>
            <div className="flex gap-1">
              {[1, 2, 3, 4, 5].map((n) => (
                <span
                  key={n}
                  className={cn(
                    "h-2.5 flex-1 rounded-full",
                    n <= d.value ? "bg-home-amber" : n - 0.5 === d.value ? "bg-home-amber/45" : "bg-surface-2",
                  )}
                />
              ))}
            </div>
            <span className="text-right font-medium">{d.value}</span>
          </div>
        ))}
      </div>
      <p className="mt-6 pt-5 border-t border-divider text-xs uppercase tracking-wider text-faint mb-2.5">
        {t("home_visual_antipatterns_clear")}
      </p>
      <div className="flex flex-wrap gap-2">
        {ANTI_PATTERNS.map((k) => (
          <span
            key={k}
            className="inline-flex items-center gap-1.5 rounded-full bg-home-green-soft text-home-green px-2.5 py-1 text-xs font-medium"
          >
            <ShieldCheck size={12} />
            {t(k)}
          </span>
        ))}
      </div>
    </div>
  );
}
