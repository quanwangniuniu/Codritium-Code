"use client";

import type { ProblemDetail } from "@/features/problems/api";
import { workspaceApi } from "@/features/workspace/api";
import { deleteSession } from "@/features/workspace/lib/session-store";
import { t } from "@/shared/i18n";
import { toast } from "@/shared/lib/toast";

interface SubmitOptions {
  problem: ProblemDetail | null;
  slug: string;
  handle: string | null;
  sessionId: string | null;
  fileContents: Record<string, string>;
}

// Submits the workspace for grading, drops the local session and moves to
// the submission report.
export function useSubmitSolution({ problem, slug, handle, sessionId, fileContents }: SubmitOptions) {
  return async () => {
    if (!problem) return;
    try {
      const data = await workspaceApi.createSubmission({
        problemSlug: slug,
        variant: problem.variant,
        codeFiles: fileContents,
        sessionId,
      });
      if (handle) deleteSession(handle, slug);
      window.location.href = `/submissions/${data.id}`;
    } catch (e) {
      console.error("submit error:", e);
      toast.error(t("submit_failed"));
    }
  };
}
