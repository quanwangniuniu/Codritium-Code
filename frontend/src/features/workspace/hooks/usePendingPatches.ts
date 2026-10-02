"use client";

import { useCallback, useRef, useState } from "react";
import type { DecisionKind } from "@/features/workspace/api";
import {
  applyToolResult,
  markPatchResolved,
} from "@/features/workspace/lib/workspace-state";
import type { PendingPatch } from "@/features/workspace/types";
import type { SetSessionFn } from "./useProblemSession";

// Tool proposals awaiting an approve / modify / reject decision, plus the
// session update when one is resolved from the UI (PatchPreview).
export function usePendingPatches(setSession: SetSessionFn) {
  const [pending, setPending] = useState<PendingPatch[]>([]);
  const draftContentRef = useRef<Record<string, string>>({});

  const addPending = useCallback((patch: PendingPatch) => {
    if (patch.newContent !== undefined) {
      draftContentRef.current[patch.toolUseId] = patch.newContent;
    }

    setPending((prev) => [...prev, patch]);
  }, []);

  const dropPending = useCallback((toolUseId: string) => {
    delete draftContentRef.current[toolUseId];
    setPending((prev) => prev.filter((p) => p.toolUseId !== toolUseId));
  }, []);

  const updatePendingContent = useCallback(
    (toolUseId: string, content: string) => {
      draftContentRef.current[toolUseId] = content;

      setPending((prev) =>
        prev.map((patch) =>
          patch.toolUseId === toolUseId
            ? { ...patch, newContent: content }
            : patch,
        ),
      );
    },
    [],
  );

  const getPendingContent = useCallback((toolUseId: string) => {
    return draftContentRef.current[toolUseId];
  }, []);

  // Records the user's decision and stages approved FileEdit content.
  // The content is applied only after a successful backend tool result.
  // If that result arrived first, apply the staged change immediately.
  const resolvePatch = useCallback(
    (
      toolUseId: string,
      kind: DecisionKind,
      result: { path?: string; content?: string } | null,
    ) => {
      dropPending(toolUseId);

      const approvedChange =
        kind !== "reject" && result?.path && result.content !== undefined
          ? {
              path: result.path,
              content: result.content,
            }
          : undefined;

      setSession((prev) => {
        const messages = markPatchResolved(
          prev.messages,
          toolUseId,
          kind,
          approvedChange,
        );

        const patchMessage = messages.find(
          (message) =>
            message.kind === "patch" && message.pending.toolUseId === toolUseId,
        );

        const nextSession = {
          ...prev,
          messages,
        };

        if (patchMessage?.kind === "patch" && patchMessage.toolResult) {
          return applyToolResult(
            nextSession,
            toolUseId,
            patchMessage.toolResult,
          );
        }

        return nextSession;
      });
    },
    [dropPending, setSession],
  );

  return {
    pending,
    addPending,
    dropPending,
    updatePendingContent,
    getPendingContent,
    resolvePatch,
  };
}
