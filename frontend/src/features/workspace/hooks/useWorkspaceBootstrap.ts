"use client";

import { useEffect, useState } from "react";
import { problemsApi, type ProblemDetail } from "@/features/problems/api";
import { workspaceApi } from "@/features/workspace/api";
import { getSession } from "@/features/workspace/lib/session-store";
import type { OpenTab } from "@/features/workspace/types";
import { t } from "@/shared/i18n";
import { toast } from "@/shared/lib/toast";
import type { SetSessionFn } from "./useProblemSession";

// Loads the problem, seeds the local session with the starter files (on a
// fresh or forced-fresh start) and opens / resumes the backend session.
export function useWorkspaceBootstrap(
  handle: string | null,
  slug: string,
  forceFresh: boolean,
  setSession: SetSessionFn,
): { problem: ProblemDetail | null; sessionId: string | null } {
  const [problem, setProblem] = useState<ProblemDetail | null>(null);
  const [sessionId, setSessionId] = useState<string | null>(null);

  useEffect(() => {
    if (!handle || !slug) return;
    let cancelled = false;
    problemsApi
      .getProblem(slug)
      .then((p) => {
        if (cancelled) return;
        setProblem(p);
        const current = getSession(handle, slug);
        const sessionIsEmpty = Object.keys(current.fileContents).length === 0;
        if (sessionIsEmpty || forceFresh) {
          const firstFile = Object.keys(p.starter_files)[0] || null;
          const initialTabs: OpenTab[] = [
            { name: "README.md", dirty: false },
            ...(firstFile ? [{ name: firstFile, dirty: false }] : []),
          ];
          const activeTab = "README.md";
          setSession((prev) => ({
            ...prev,
            fileContents: p.starter_files,
            originalContents: p.starter_files,
            openTabs: initialTabs,
            activeTab,
            editorPanes: [{ id: "main", tabs: initialTabs, activeTab }],
            messages: [],
          }));
        } else {
          setSession((prev) => ({ ...prev, originalContents: p.starter_files }));
        }
        return workspaceApi.createSession(slug, forceFresh);
      })
      .then((s) => {
        if (cancelled || !s) return;
        setSessionId(s.session_id);
      })
      .catch((e) => {
        console.error("workspace bootstrap:", e);
        toast.error(t("submit_failed"));
      });
    return () => {
      cancelled = true;
    };
  }, [slug, handle, setSession, forceFresh]);

  return { problem, sessionId };
}
