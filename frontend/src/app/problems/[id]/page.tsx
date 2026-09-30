import Link from "next/link";
import { notFound, redirect } from "next/navigation";
import {
  ArrowLeft,
  Building2,
  CircleCheck,
  CircleDashed,
  FileText,
  FlaskConical,
  History,
  Lock,
  MessageSquare,
  Sparkles,
  Timer,
} from "lucide-react";
import { getProblem, listMyAttemptedProblems, listUserProblemSubmissions } from "@/lib/store";
import { currentUser } from "@/lib/auth";
import { t, type LocaleKey } from "@/lib/i18n";
import { splitProblemReadme } from "@/lib/problem-readme";
import { cn } from "@/lib/utils";
import { CATEGORY_LABEL_KEY, DIFFICULTY_LABEL_KEY, DIFFICULTY_TONE } from "@/shared/labels";
import { formatDateTime } from "@/shared/format";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Markdown } from "@/components/markdown";
import { ResumeOrFresh } from "@/components/ResumeOrFresh";
import { CommentsPanel } from "@/components/CommentsPanel";
import { ProblemSplitView } from "@/components/ProblemSplitView";
import type { Submission } from "@/lib/types";

const SUBMISSION_STATUS_KEY: Record<Exclude<Submission["status"], "completed">, LocaleKey> = {
  pending: "problem_submission_status_pending",
  grading: "problem_submission_status_grading",
  failed: "problem_submission_status_failed",
};

interface ProblemIntroProps {
  params: Promise<{ id: string }>;
}

export default async function ProblemIntroPage({ params }: ProblemIntroProps) {
  const { id } = await params;
  const user = await currentUser();
  if (!user) redirect(`/login?next=${encodeURIComponent(`/problems/${id}`)}`);

  const [problem, submissions, attempted] = await Promise.all([
    getProblem(id),
    listUserProblemSubmissions(user.id, id),
    listMyAttemptedProblems(),
  ]);
  if (!problem) notFound();

  const { company, body } = splitProblemReadme(problem.description_md);
  const solved = submissions.some((s) => s.status === "completed");
  const started = solved || attempted.some((a) => a.slug === id);

  const description = (
    <div className="space-y-4">
      <h1 className="flex items-start gap-2 text-2xl font-semibold tracking-tight">
        <span className="flex-1">{problem.title}</span>
        {solved ? (
          <span className="mt-1.5 flex shrink-0 items-center gap-1 text-xs font-medium text-success">
            <CircleCheck size={15} />
            {t("problems_status_solved")}
          </span>
        ) : started ? (
          <span className="mt-1.5 flex shrink-0 items-center gap-1 text-xs font-medium text-warning">
            <CircleDashed size={15} />
            {t("problems_status_attempted")}
          </span>
        ) : null}
      </h1>
      <div className="flex flex-wrap items-center gap-1.5">
        <Badge tone={DIFFICULTY_TONE[problem.difficulty]}>
          {t(DIFFICULTY_LABEL_KEY[problem.difficulty])}
        </Badge>
        <Badge tone="neutral">{t(CATEGORY_LABEL_KEY[problem.category])}</Badge>
        {company && (
          <Badge tone="neutral" className="gap-1">
            <Building2 size={12} />
            {company}
          </Badge>
        )}
      </div>
      <Markdown source={body} className="text-sm leading-relaxed text-ink" />
    </div>
  );

  const solutions = (
    <div className="flex flex-col items-center gap-3 py-12 text-center">
      {solved ? (
        <>
          <p className="max-w-sm text-sm text-muted">{t("problem_solutions_unlocked_desc")}</p>
          <Link href={`/problems/${id}/reply`}>
            <Button className="gap-2">
              <FlaskConical size={14} />
              {t("problem_solutions_view_btn")}
            </Button>
          </Link>
        </>
      ) : (
        <>
          <Lock size={20} className="text-faint" />
          <p className="max-w-sm text-sm text-muted">{t("problem_solutions_desc")}</p>
        </>
      )}
    </div>
  );

  const submissionsList =
    submissions.length === 0 ? (
      <p className="py-12 text-center text-sm text-muted">{t("problem_submissions_empty")}</p>
    ) : (
      <ul className="divide-y divide-divider overflow-hidden rounded-md border border-divider">
        {submissions.map((s) => (
          <li key={s.id}>
            <Link
              href={`/submissions/${s.id}`}
              className="flex items-center justify-between gap-4 px-4 py-3 text-sm transition-colors hover:bg-surface-2"
            >
              {s.status === "completed" ? (
                <span className="font-medium text-success">
                  {Math.round(s.score?.total ?? 0)}{" "}
                  <span className="font-normal text-muted">{t("submission_total_out_of_100")}</span>
                </span>
              ) : (
                <span
                  className={cn(
                    "font-medium",
                    s.status === "failed" ? "text-danger" : "text-warning",
                  )}
                >
                  {t(SUBMISSION_STATUS_KEY[s.status])}
                </span>
              )}
              <time dateTime={s.submitted_at} className="text-xs text-muted tabular-nums">
                {formatDateTime(s.submitted_at)}
              </time>
            </Link>
          </li>
        ))}
      </ul>
    );

  const startPane = (
    <div className="flex h-full flex-col gap-6 p-6 sm:p-8">
      <Link href="/problems" className="btn-pill btn-pill--ghost self-start text-sm">
        <ArrowLeft size={15} strokeWidth={1.8} />
        {t("back_to_problems")}
      </Link>
      <div className="my-auto space-y-6">
        <div className="space-y-2">
          <h2 className="text-xl font-semibold tracking-tight">{t("problem_start_title")}</h2>
          <p className="text-sm leading-relaxed text-muted">{t("problem_intro_blurb")}</p>
        </div>
        <ul className="space-y-3 text-sm">
          <StartPoint icon={<FileText size={16} />} text={t("problem_start_point_files")} />
          <StartPoint icon={<Sparkles size={16} />} text={t("problem_start_point_ai")} />
          <StartPoint icon={<Timer size={16} />} text={t("problem_start_point_timer")} />
        </ul>
        <div className="hidden lg:block">
          <ResumeOrFresh slug={id} align="start" />
        </div>
      </div>
    </div>
  );

  return (
    <>
      <ProblemSplitView
        tabs={[
          {
            id: "description",
            label: t("problem_tab_description"),
            icon: <FileText size={14} />,
            content: description,
          },
          {
            id: "solutions",
            label: t("problem_tab_solutions"),
            icon: solved ? <FlaskConical size={14} /> : <Lock size={14} />,
            content: solutions,
          },
          {
            id: "discussion",
            label: t("problem_tab_discussion"),
            icon: <MessageSquare size={14} />,
            content: <CommentsPanel problemSlug={id} currentUserId={user.id} embedded />,
          },
          {
            id: "submissions",
            label: t("problem_tab_submissions"),
            icon: <History size={14} />,
            content: submissionsList,
          },
        ]}
        side={startPane}
      />
      {/* Narrow screens stack the panes, so keep the start action pinned. */}
      <div className="sticky bottom-0 z-20 border-t border-divider bg-canvas/90 px-4 py-3 backdrop-blur lg:hidden">
        <ResumeOrFresh slug={id} align="start" />
      </div>
    </>
  );
}

function StartPoint({ icon, text }: { icon: React.ReactNode; text: string }) {
  return (
    <li className="flex items-start gap-3 text-ink">
      <span className="mt-0.5 shrink-0 text-accent">{icon}</span>
      {text}
    </li>
  );
}
