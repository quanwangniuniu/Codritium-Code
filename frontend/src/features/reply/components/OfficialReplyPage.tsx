import Link from "next/link";
import { notFound, redirect } from "next/navigation";
import { Lock } from "lucide-react";
import { getProblem } from "@/features/problems/server";
import { getOfficialReply } from "@/features/reply/server";
import { currentUser } from "@/features/auth/server";
import { ReplyClient } from "@/features/reply/components/ReplyClient";
import { CommentsPanel } from "@/features/problems/components/CommentsPanel";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/shared/ui/Card";
import { Button } from "@/shared/ui/Button";
import { t } from "@/shared/i18n";
import { transformForReplay, type ReplyEnvelope } from "@/features/reply/types";

interface OfficialReplyPageProps {
  params: Promise<{ id: string }>;
}

// Official solution walkthrough for a problem (per-problem, not per-submission).
// Distinct from a candidate's own run at /submissions/[id]/reply. The backend
// gates the official-reply endpoint behind a graded submission, so a null
// payload here means "not unlocked yet".
export async function OfficialReplyPage({ params }: OfficialReplyPageProps) {
  const { id } = await params;
  const user = await currentUser();
  if (!user) redirect(`/login?next=${encodeURIComponent(`/problems/${id}/reply`)}`);

  const [officialRaw, problem] = await Promise.all([
    getOfficialReply(id),
    getProblem(id),
  ]);
  if (!problem) notFound();

  if (!officialRaw) {
    return (
      <div className="mx-auto max-w-3xl px-6 py-16">
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2 text-base">
              <Lock size={15} strokeWidth={1.8} />
              {t("official_locked_title")}
            </CardTitle>
            <CardDescription>{t("official_locked_body")}</CardDescription>
          </CardHeader>
          <CardContent>
            <Link href={`/problems/${id}`}>
              <Button variant="outline" size="sm">
                {t("official_back_link")}
              </Button>
            </Link>
          </CardContent>
        </Card>
      </div>
    );
  }

  const data = {
    envelopes: transformForReplay(officialRaw.envelopes as ReplyEnvelope[]),
    files: officialRaw.files ?? {},
    starterFiles: flattenStarterFiles(officialRaw.starter_files ?? {}),
    explanationMd: officialRaw.explanation_md ?? "",
  };

  return (
    <div className="space-y-4">
      <ReplyClient
        variant="official"
        title={t("official_solution_title")}
        subtitle={t("official_solution_subtitle")}
        backHref={`/problems/${id}`}
        backLabel={t("official_back_link")}
        data={data}
        emptyReplay={t("reply_no_official_short")}
        emptyAnswer={t("reply_empty_official_full")}
      />
      <CommentsPanel problemSlug={id} currentUserId={user.id} />
    </div>
  );
}

function flattenStarterFiles(raw: Record<string, unknown>): Record<string, string> {
  const out: Record<string, string> = {};
  for (const [name, value] of Object.entries(raw)) {
    if (typeof value === "string") {
      out[name] = value;
      continue;
    }
    if (value && typeof value === "object") {
      const obj = value as Record<string, unknown>;
      const candidate = obj["as-is"] ?? obj.stripped;
      if (typeof candidate === "string") {
        out[name] = candidate;
      }
    }
  }
  return out;
}
