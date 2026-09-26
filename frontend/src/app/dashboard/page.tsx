import Link from "next/link";
import { redirect } from "next/navigation";
import { currentUser } from "@/lib/auth";
import { listProblems, listUserSubmissions } from "@/lib/store";
import { t, type LocaleKey } from "@/lib/i18n";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { SubmissionsList } from "@/components/SubmissionsList";
import { CalendarHeatmap } from "@/components/calendar-heatmap";
import { CategoryMastery } from "@/components/category-mastery";
import { TotalScoreCircle } from "@/components/total-score-circle";
import { StreakFlame } from "@/components/streak-flame";
import type { Category, Problem } from "@/lib/types";

const CATEGORIES: { value: "all" | Category; labelKey: LocaleKey }[] = [
  { value: "all", labelKey: "profile_category_all" },
  { value: "debugging", labelKey: "profile_category_debugging" },
  { value: "refactoring", labelKey: "profile_category_refactoring" },
  { value: "feature_build", labelKey: "profile_category_feature_build" },
  { value: "security", labelKey: "profile_category_security" },
  { value: "company_premium", labelKey: "profile_category_company_premium" },
];

interface DashboardPageProps {
  searchParams: Promise<{ category?: string }>;
}

export default async function DashboardPage({ searchParams }: DashboardPageProps) {
  const user = await currentUser();
  if (!user) redirect("/login");
  const sp = await searchParams;
  const filter = (sp.category ?? "all") as "all" | Category;

  const submissions = await listUserSubmissions(user.id);
  const problems = await listProblems();
  const problemMap = new Map(problems.map((p) => [p.id, p]));
  const problemMapObj: Record<string, Problem> = Object.fromEntries(
    problems.map((p) => [p.id, p]),
  );

  const scored = submissions.filter((s) => s.score);
  const avgScore =
    scored.length === 0
      ? 0
      : scored.reduce((a, s) => a + (s.score?.total ?? 0), 0) / scored.length;

  const filtered =
    filter === "all"
      ? submissions
      : submissions.filter((s) => problemMap.get(s.problem_id)?.category === filter);

  return (
    <div className="mx-auto max-w-[1600px] px-6 py-10 space-y-8">
      <header className="flex items-end justify-between flex-wrap gap-3">
        <div className="space-y-1">
          <h1 className="text-3xl tracking-tight font-semibold">{t("nav_dashboard")}</h1>
          <p className="text-sm text-muted">{t("dashboard_subtitle")}</p>
        </div>
        <StreakFlame count={user.streak_current} size="lg" />
      </header>

      <div className="grid gap-3 sm:grid-cols-3">
        <Card className="flex items-center gap-4 px-4 py-4">
          <TotalScoreCircle total={avgScore} size={104} />
          <div className="min-w-0">
            <div className="text-xs uppercase tracking-wider text-faint">
              {t("profile_total_label")}
            </div>
            <div className="text-sm text-muted tabular-nums">
              {scored.length} graded
            </div>
          </div>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription className="text-xs uppercase tracking-wider">
              {t("profile_current_label")}
            </CardDescription>
            <CardTitle className="text-2xl tabular-nums">{user.streak_current}</CardTitle>
          </CardHeader>
          <CardContent className="pt-0">
            <StreakFlame count={user.streak_current} size="sm" />
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription className="text-xs uppercase tracking-wider">
              {t("streak_longest_label")}
            </CardDescription>
            <CardTitle className="text-2xl tabular-nums">{user.streak_longest}</CardTitle>
          </CardHeader>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="text-lg">{t("profile_activity_title")}</CardTitle>
          <CardDescription className="leading-relaxed">
            {t("profile_activity_desc")}
          </CardDescription>
        </CardHeader>
        <CardContent>
          <CalendarHeatmap submissions={submissions} />
        </CardContent>
      </Card>

      <section className="space-y-3">
        <h2 className="text-xl tracking-tight font-semibold">
          {t("profile_category_mastery_label")}
        </h2>
        <CategoryMastery submissions={submissions} problems={problems} />
      </section>

      <section className="space-y-3">
        <div className="flex items-end justify-between flex-wrap gap-3">
          <h2 className="text-xl tracking-tight font-semibold">
            {t("profile_submission_history_title")}
          </h2>
          <Link href="/problems">
            <Button variant="outline" size="sm">
              {t("profile_find_problem_btn")}
            </Button>
          </Link>
        </div>
        <div className="flex flex-wrap gap-1.5">
          {CATEGORIES.map((c) => {
            const active = filter === c.value;
            return (
              <Link
                key={c.value}
                href={c.value === "all" ? "/dashboard" : `/dashboard?category=${c.value}`}
                className={
                  active
                    ? "px-2.5 py-1 text-xs rounded-full bg-accent text-accent-fg border border-accent"
                    : "px-2.5 py-1 text-xs rounded-full border border-divider text-muted hover:text-ink hover:border-divider-strong"
                }
              >
                {t(c.labelKey)}
              </Link>
            );
          })}
        </div>
        <SubmissionsList initialSubmissions={filtered} problemMap={problemMapObj} />
      </section>
    </div>
  );
}
