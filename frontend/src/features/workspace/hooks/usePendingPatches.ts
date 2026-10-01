"use client";

import { useCallback, useState } from "react";
import type { DecisionKind } from "@/features/workspace/api";
import { applyFileChange, markPatchResolved } from "@/features/workspace/lib/workspace-state";
import type { PendingPatch } from "@/features/workspace/types";
import type { SetSessionFn } from "./useProblemSession";

// Tool proposals awaiting an approve / modify / reject decision, plus the
// session update when one is resolved from the UI (PatchPreview).
export function usePendingPatches(setSession: SetSessionFn) {
  const [pending, setPending] = useState<PendingPatch[]>([]);

  const addPending = useCallback((patch: PendingPatch) => {
    setPending((prev) => [...prev, patch]);
  }, []);

  const dropPending = useCallback((toolUseId: string) => {
    setPending((prev) => prev.filter((p) => p.toolUseId !== toolUseId));
  }, []);

  // The decision already reached the backend; mirror it locally and, for
  // approve / modify, write the landed content into the workspace.
  const resolvePatch = useCallback(
    (toolUseId: string, kind: DecisionKind, result: { path?: string; content?: string } | null) => {
      dropPending(toolUseId);
      setSession((prev) => {
        const withResolved = { ...prev, messages: markPatchResolved(prev.messages, toolUseId, kind) };
        if (kind !== "reject" && result?.path && result.content !== undefined) {
          return applyFileChange(withResolved, result.path, result.content);
        }
        return withResolved;
      });
    },
    [dropPending, setSession],
  );

  return { pending, addPending, dropPending, resolvePatch };
}
