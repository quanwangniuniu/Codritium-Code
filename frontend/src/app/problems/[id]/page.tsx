import Link from "next/link";
import { notFound, redirect } from "next/navigation";
import { ArrowLeft, Lock, MessageSquare } from "lucide-react";
import { getProblem } from "@/lib/store";
import { currentUser } from "@/lib/auth";
import { t } from "@/lib/i18n";
import { Breadcrumbs } from "@/components/breadcrumbs";
import { Badge } from "@/components/ui/badge";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Markdown } from "@/components/markdown";
import { ResumeOrFresh } from "@/components/ResumeOrFresh";

interface ProblemIntroProps {
  params: Promise<{ id: string }>;
}

export default async function ProblemIntroPage({ params }: ProblemIntroProps) {
  const { id } = await params;
  const user = await currentUser();
  if (!user) redirect(`/login?next=${encodeURIComponent(`/problems/${id}`)}`);

  const problem = await getProblem(id);
  if (!problem) notFound();

  return (
    <div className="mx-auto max-w-4xl px-6 py-8 space-y-6">
      <div className="space-y-2">
        <Link
          href="/problems"
          className="btn-pill btn-pill--ghost text-sm"
        >
          <ArrowLeft size={15} strokeWidth={1.8} />
          {t("back_to_problems")}
        </Link>
        <Breadcrumbs
          items={[
            { label: t("breadcrumb_problems"), href: "/problems" },
            { label: problem.title },
          ]}
        />
      </div>

      <header className="space-y-3">
        <div className="flex items-start justify-between gap-4 flex-wrap">
          <div className="space-y-2">
            <h1 className="text-3xl tracking-tight font-semibold">
              {problem.title}
            </h1>
            <div className="flex items-center gap-1.5 flex-wrap">
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
            </div>
          </div>
          <ResumeOrFresh slug={id} align="end" />
        </div>
        <p className="text-sm text-muted leading-relaxed max-w-2xl">
          {t("problem_intro_blurb")}
        </p>
      </header>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">{t("problem_brief_card_title")}</CardTitle>
          <CardDescription>{t("problem_brief_card_desc")}</CardDescription>
        </CardHeader>
        <CardContent>
          <Markdown
            source={problem.description_md}
            className="text-sm leading-relaxed text-ink"
          />
        </CardContent>
      </Card>

      <div className="grid md:grid-cols-2 gap-4">
        <Link href={`/problems/${id}/reply`} className="group block">
          <Card className="transition-colors group-hover:border-divider-strong">
            <CardHeader>
              <div className="flex items-center justify-between">
                <CardTitle className="text-sm flex items-center gap-2">
                  <Lock size={14} strokeWidth={1.7} />
                  {t("problem_solutions_title")}
                </CardTitle>
                <Badge tone="neutral">{t("badge_locked")}</Badge>
              </div>
              <CardDescription>{t("problem_solutions_desc")}</CardDescription>
            </CardHeader>
          </Card>
        </Link>

        <Card>
          <CardHeader>
            <div className="flex items-center justify-between">
              <CardTitle className="text-sm flex items-center gap-2">
                <MessageSquare size={14} strokeWidth={1.7} />
                {t("problem_discussion_title")}
              </CardTitle>
              <Badge tone="neutral">{t("badge_soon")}</Badge>
            </div>
            <CardDescription>{t("problem_discussion_desc")}</CardDescription>
          </CardHeader>
        </Card>
      </div>

    </div>
  );
}
