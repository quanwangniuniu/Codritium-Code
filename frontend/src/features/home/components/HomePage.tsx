import { Code2 } from "lucide-react";
import { AudienceSection } from "@/features/home/components/AudienceSection";
import { CodeShowcase, type ShowcaseTrack } from "@/features/home/components/CodeShowcase";
import { ExploreSection, type CategoryStat } from "@/features/home/components/ExploreSection";
import { HomeFooter } from "@/features/home/components/HomeFooter";
import { HomeHero } from "@/features/home/components/HomeHero";
import { MissionSection } from "@/features/home/components/MissionSection";
import { ACCENT_TEXT, SectionBadge, type HomeAccent } from "@/features/home/components/SectionBadge";
import { currentUser } from "@/features/auth/server";
import { t, type LocaleKey } from "@/shared/i18n";
import { getProblem, listProblems } from "@/features/problems/server";
import type { Category, Difficulty, Problem } from "@/features/problems/types";
import { cn } from "@/shared/lib/cn";
import { DIFFICULTIES, DIFFICULTY_LABEL_KEY } from "@/shared/labels";

const DIFFICULTY_OPTIONS = DIFFICULTIES.map((value) => ({ value, labelKey: DIFFICULTY_LABEL_KEY[value] }));

const TRACKS: { category: Category; labelKey: LocaleKey; accent: HomeAccent }[] = [
  { category: "debugging", labelKey: "profile_category_debugging", accent: "blue" },
  { category: "security", labelKey: "profile_category_security", accent: "rose" },
  { category: "refactoring", labelKey: "profile_category_refactoring", accent: "teal" },
  { category: "feature_build", labelKey: "profile_category_feature_build", accent: "green" },
  { category: "company_premium", labelKey: "profile_category_company_premium", accent: "amber" },
];

const SHOWCASE_PER_TRACK = 3;
const SHOWCASE_TRACKS = 3;

// The file a candidate would open first: the first non-test source file.
function mainFile(files: Record<string, string>): string | null {
  const names = Object.keys(files).sort();
  return names.find((n) => !/(^|\/)test_|_test\.|\.test\.|\.md$/.test(n)) ?? names[0] ?? null;
}

async function loadShowcase(problems: Problem[]): Promise<ShowcaseTrack[]> {
  const rank: Record<Difficulty, number> = { easy: 0, medium: 1, hard: 2 };
  const picks = TRACKS.map((tr) => ({
    label: t(tr.labelKey),
    problems: problems
      .filter((p) => p.category === tr.category && !p.is_pro_only)
      .sort((a, b) => rank[a.difficulty] - rank[b.difficulty])
      .slice(0, SHOWCASE_PER_TRACK),
  }))
    .filter((tr) => tr.problems.length > 0)
    .slice(0, SHOWCASE_TRACKS);

  const tracks = await Promise.all(
    picks.map(async (tr) => {
      const full = await Promise.all(tr.problems.map((p) => getProblem(p.id)));
      return {
        label: tr.label,
        problems: full.flatMap((p) => {
          const file = p ? mainFile(p.starter_files) : null;
          return p && file ? [{ slug: p.id, title: p.title, file, code: p.starter_files[file] }] : [];
        }),
      };
    }),
  );
  return tracks.filter((tr) => tr.problems.length > 0);
}

export async function HomePage() {
  const [user, problems] = await Promise.all([currentUser(), listProblems()]);

  const categories: CategoryStat[] = TRACKS.map((tr) => ({
    labelKey: tr.labelKey,
    accent: tr.accent,
    count: problems.filter((p) => p.category === tr.category).length,
  })).filter((c) => c.count > 0);
  const difficulties = DIFFICULTY_OPTIONS.map((d) => ({
    labelKey: d.labelKey,
    count: problems.filter((p) => p.difficulty === d.value).length,
  }));
  const showcase = await loadShowcase(problems);

  return (
    <div>
      <HomeHero signedIn={!!user} problemCount={problems.length} trackCount={categories.length} />
      <ExploreSection categories={categories} difficulties={difficulties} />
      <AudienceSection problemCount={problems.length} />

      {showcase.length > 0 && (
        <section className="mx-auto max-w-[1200px] px-4 sm:px-6 py-20 lg:py-24 text-center">
          <SectionBadge icon={Code2} accent="teal" />
          <h2 className={cn("mt-6 text-3xl sm:text-4xl font-semibold tracking-tight", ACCENT_TEXT.teal)}>
            {t("home_showcase_title")}
          </h2>
          <p className="mt-5 max-w-2xl mx-auto text-muted leading-relaxed mb-12">{t("home_showcase_blurb")}</p>
          <CodeShowcase tracks={showcase} />
        </section>
      )}

      <div className="bg-surface border-t border-divider">
        <MissionSection />
        <HomeFooter />
      </div>
    </div>
  );
}
