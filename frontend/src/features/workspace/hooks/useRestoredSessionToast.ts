"use client";

import { useEffect } from "react";
import { clearRestoredFlag } from "@/features/workspace/lib/session-store";
import { t } from "@/shared/i18n";
import { toast } from "@/shared/lib/toast";

// One-shot "restored from your previous session" toast when the local
// session was hydrated from storage with real work in it.
export function useRestoredSessionToast(
  restored: boolean | undefined,
  handle: string | null,
  slug: string,
): void {
  useEffect(() => {
    if (!restored || !handle) return;
    toast.info(t("restored_from_session"));
    clearRestoredFlag(handle, slug);
  }, [restored, handle, slug]);
}
