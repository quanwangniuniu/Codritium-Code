import Link from "next/link";
import { notFound, redirect } from "next/navigation";
import { RotateCcw, BookOpen } from "lucide-react";
import {
  getProblem,
  getSubmission,
  listUserProblemSubmissions,
} from "@/lib/store";
import { currentUser } from "@/lib/auth";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Breadcrumbs } from "@/components/breadcrumbs";
import { ScoreBar } from "@/components/score-bar";
import { AntiPatternChip } from "@/components/anti-pattern-chip";
import { SubmissionTabs } from "@/components/SubmissionTabs";
import { cn } from "@/lib/utils";
import { t } from "@/shared/i18n";
import { DIMENSION_ORDER, dimensionLabel } from "@/shared/labels";
import { formatDate, formatDateTime } from "@/shared/format";

interface SubmissionPageProps {
  params: Promise<{ id: string }>;
}

export default async function SubmissionPage({ params }: SubmissionPageProps) {
  const { id } = await params;
  const user = await currentUser();
  if (!user) redirect("/login");

  const submission = await getSubmission(id);
  if (!submission) notFound();
  if (submission.user_id !== user.id) redirect("/profile");

  const problem = await getProblem(submission.problem_id);
  if (!problem) notFound();

  const iterations = await listUserProblemSubmissions(user.id, problem.id);
  const currentIdx = iterations.findIndex((s) => s.id === submission.id);

  const score = submission.score;
  const flags = submission.anti_patterns;

  // The scored report only exists once grading completes. A pending / grading
  // / failed submission arrives with a null score; render its status instead
  // of dereferencing score.total.
  if (!score || !flags) {
    const grading =
      submission.status === "pending" || submission.status === "grading";
    return (
      <div className="mx-auto max-w-[1600px] px-6 py-6 space-y-4">
        <SubmissionTabs submissionId={submission.id} active="report" />
        <Breadcrumbs
          items={[
            { label: t("breadcrumb_problems"), href: "/problems" },
            { label: problem.title, href: `/problems/${problem.id}` },
            { label: `Submission #${iterations.length - currentIdx}` },
          ]}
        />
        <Card>
          <CardHeader>
            <div className="flex items-center gap-2">
              <CardTitle className="text-base">
                {t("submission_not_graded_title")}
              </CardTitle>
              <Badge tone={grading ? "info" : "danger"}>{submission.status}</Badge>
            </div>
            <CardDescription>
              {grading ? t("submission_grading_body") : t("submission_failed_body")}
            </CardDescription>
          </CardHeader>
          <CardContent>
            <Link href={`/problems/${problem.id}`}>
              <Button variant="outline" size="sm">
                <RotateCcw size={12} className="mr-1.5" />
                {t("submission_resubmit_btn")}
              </Button>
            </Link>
          </CardContent>
        </Card>
      </div>
    );
  }

  const flagValues = Object.values(flags);
  const flaggedCount = flagValues.filter(
    (flag) => flag.evaluated && flag.triggered,
  ).length;
  const evaluatedCount = flagValues.filter((flag) => flag.evaluated).length;

  return (
    <div className="mx-auto max-w-[1600px] px-6 py-6 space-y-4">
      <SubmissionTabs submissionId={submission.id} active="report" />
      <div className="flex items-start justify-between gap-4">
        <Breadcrumbs
          items={[
            { label: t("breadcrumb_problems"), href: "/problems" },
            { label: problem.title, href: `/problems/${problem.id}` },
            { label: `Submission #${iterations.length - currentIdx}` },
          ]}
        />
        <Link href={`/problems/${problem.id}/reply`}>
          <Button variant="outline" size="sm" className="inline-flex items-center gap-1.5">
            <BookOpen size={14} strokeWidth={2} />
            {t("view_official_solution")}
          </Button>
        </Link>
      </div>

      <div className="grid lg:grid-cols-[220px_1fr] gap-6 items-start">
        <aside className="lg:sticky lg:top-20 space-y-3">
          <div className="flex items-center justify-between">
            <h2 className="text-xs uppercase tracking-wider text-faint">{t("submission_iterations_title")}</h2>
            <span className="text-xs text-faint tabular-nums">{iterations.length}</span>
          </div>
          <div className="space-y-1.5">
            {iterations.map((s, i) => {
              const total = s.score?.total ?? 0;
              const prev = iterations[i + 1];
              const delta = prev?.score ? total - prev.score.total : null;
              const active = s.id === submission.id;
              return (
                <Link
                  key={s.id}
                  href={`/submissions/${s.id}`}
                  className={cn(
                    "block rounded-md border px-2.5 py-2 transition-colors",
                    active
                      ? "border-accent bg-accent-soft"
                      : "border-divider bg-surface hover:border-divider-strong",
                  )}
                >
                  <div className="flex items-center gap-2">
                    <span
                      className={cn(
                        "text-base font-semibold tabular-nums w-8 tracking-tight",
                        active ? "text-ink" : "text-muted",
                      )}
                    >
                      {total.toFixed(0)}
                    </span>
                    <div className="flex-1 min-w-0">
                      <div className={cn("text-xs", active ? "text-ink" : "text-muted")}>
                        #{iterations.length - i}
                      </div>
                      <div className="text-[10px] text-faint truncate">
                        {formatDate(s.submitted_at)}
                      </div>
                    </div>
                    {delta !== null && (
                      <span
                        className={cn(
                          "text-[10px] tabular-nums",
                          delta > 0 ? "text-success" : delta < 0 ? "text-danger" : "text-faint",
                        )}
                      >
                        {delta > 0 ? "+" : ""}
                        {delta.toFixed(0)}
                      </span>
                    )}
                  </div>
                </Link>
              );
            })}
          </div>
          <Link href={`/problems/${problem.id}`} className="block">
            <Button variant="outline" size="sm" className="w-full">
              <RotateCcw size={12} className="mr-1.5" />
              {t("submission_resubmit_btn")}
            </Button>
          </Link>
        </aside>

        <div className="space-y-6">
          <header className="grid sm:grid-cols-[auto_1fr] gap-6 items-center">
            <div className="flex min-w-[132px] flex-col items-center justify-center">
              <span
                className={cn(
                  "text-5xl font-semibold tabular-nums tracking-tight",
                  score.total >= 80
                    ? "text-success"
                    : score.total >= 60
                      ? "text-warning"
                      : "text-danger",
                )}
              >
                {score.total.toFixed(0)}
              </span>
              <span className="mt-1 text-xs text-faint">{t("submission_total_out_of_100")}</span>
            </div>
            <div className="space-y-2">
              <div className="flex flex-wrap items-center gap-2">
                <Badge tone="info">{problem.category}</Badge>
                <Badge
                  tone={
                    problem.difficulty === "easy"
                      ? "success"
                      : problem.difficulty === "medium"
                        ? "warning"
                        : "danger"
                  }
                >
                  {problem.difficulty}
                </Badge>
                <Badge tone="neutral">{t("submission_judge_prefix")} {score.judge_model}</Badge>
              </div>
              <h1 className="text-2xl tracking-tight font-semibold">{problem.title}</h1>
              <p className="text-sm text-muted">
                {t("submission_meta_submitted_fmt", {
                  params: {
                    submitted: formatDateTime(submission.submitted_at),
                    graded: submission.graded_at ? formatDateTime(submission.graded_at) : "—",
                  },
                })}
              </p>
            </div>
          </header>

          <div className="grid lg:grid-cols-3 gap-6">
            <div className="lg:col-span-2 space-y-6">
              <Card>
                <CardHeader>
                  <CardTitle className="text-base">{t("submission_five_dim_title")}</CardTitle>
                  <CardDescription>
                    {t("submission_five_dim_desc_prefix")}
                  </CardDescription>
                </CardHeader>
                <CardContent className="space-y-5">
                  {DIMENSION_ORDER.map((dim) => {
                    const weight = score.weights_applied[dim];
                    if (weight === undefined) return null;

                    const dimScore = score[dim];
                    
                    return (
                      <ScoreBar
                        key={dim}
                        label={dimensionLabel(dim)}
                        score={dimScore.score}
                        weight={weight}
                        reasoning={dimScore.reasoning}
                      />
                    );
                  })}
                </CardContent>
              </Card>

            </div>

            <div className="space-y-6">
              <Card>
                <CardHeader>
                  <CardTitle className="text-base">{t("submission_anti_pattern_title")}</CardTitle>
                  <CardDescription>
                    {evaluatedCount === 0
                      ? t("submission_anti_pattern_unevaluated")
                      : flaggedCount === 0
                        ? t("submission_anti_pattern_clean")
                        : t("submission_anti_pattern_count_fmt", {
                          params: { n: flaggedCount },
                        })}
                  </CardDescription>
                </CardHeader>
                <CardContent className="space-y-2.5">
                  {(["hands_off", "feature_marathon", "ai_showcase", "not_thinking"] as const).map(
                    (k) => (
                      <AntiPatternChip
                        key={k}
                        name={k}
                        evaluated={flags[k].evaluated}
                        triggered={flags[k].triggered}
                        evidence={flags[k].evidence}
                      />
                    ),
                  )}
                </CardContent>
              </Card>

            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
