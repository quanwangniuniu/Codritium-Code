import { notFound, redirect } from "next/navigation";
import {
  getSubmission,
  getProblem,
  getSessionReply,
  getMyReplay,
} from "@/lib/store";
import { currentUser } from "@/lib/auth";
import { ReplyClient } from "@/components/reply/ReplyClient";
import { SubmissionTabs } from "@/components/SubmissionTabs";
import { CommentsPanel } from "@/components/CommentsPanel";
import { NotesPanel } from "@/components/NotesPanel";
import { t } from "@/shared/i18n";
import { transformForReplay, type ReplyEnvelope } from "@/types/reply";

interface ReplyPageProps {
  params: Promise<{ id: string }>;
}

// The candidate's own run for one submission. Sourced by the submission's
// session_id so multiple attempts stay distinct. The official solution is a
// separate page (/problems/[slug]/reply) reached via the header link.
export default async function MyRunReplyPage({ params }: ReplyPageProps) {
  const { id } = await params;

  const user = await currentUser();
  if (!user) redirect("/login");

  const submission = await getSubmission(id);
  if (!submission) notFound();
  if (submission.user_id !== user.id) redirect("/profile");

  const slug = submission.problem_id;

  // Prefer the run tied to this submission's session; fall back to the
  // candidate's latest run for the problem when no session is linked.
  const [mineRaw, problem] = await Promise.all([
    submission.session_id
      ? getSessionReply(submission.session_id)
      : getMyReplay(slug),
    getProblem(slug),
  ]);

  const starterFiles = mineRaw?.starter_files ?? problem?.starter_files ?? {};

  const data = mineRaw
    ? {
        envelopes: transformForReplay(mineRaw.envelopes as ReplyEnvelope[]),
        files: {},
        starterFiles,
        explanationMd: "",
      }
    : null;

  const sessionId = mineRaw?.session_id ?? submission.session_id ?? "";

  return (
    <div className="space-y-4">
      <div className="mx-auto max-w-[1600px] px-6 pt-6">
        <SubmissionTabs submissionId={id} active="reply" />
      </div>
      <ReplyClient
        variant="mine"
        title={t("reply_page_title", { params: { slug } })}
        subtitle={t("reply_subtitle")}
        backHref={`/submissions/${id}`}
        backLabel={t("reply_back_link")}
        data={data}
        emptyReplay={t("reply_empty_mine")}
        emptyAnswer={t("reply_empty_mine")}
        secondaryHref={`/problems/${slug}/reply`}
        secondaryLabel={t("view_official_solution")}
      />
      {sessionId && <NotesPanel sessionId={sessionId} currentUserId={user.id} />}
      <CommentsPanel problemSlug={slug} currentUserId={user.id} />
    </div>
  );
}
