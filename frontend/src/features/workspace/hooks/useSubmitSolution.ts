"use client";

import { useCallback, useState } from "react";

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

// Submits the workspace for grading and exposes its in-flight state so the
// UI can prevent duplicate or incomplete submissions.
export function useSubmitSolution({
  problem,
  slug,
  handle,
  sessionId,
  fileContents,
}: SubmitOptions) {
  const [submitting, setSubmitting] = useState(false);

  const submit = useCallback(async () => {
    if (!problem || submitting) return;

    setSubmitting(true);
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
      toast.error(
        e instanceof Error && e.message ? e.message : t("submit_failed"),
      );
      setSubmitting(false);
    }
  }, [
    problem,
    submitting,
    slug,
    fileContents,
    sessionId,
    handle,
  ]);

  return { submit, submitting };
}
