"use client";

import { useCallback, useEffect, useRef } from "react";

// useFocusRestore tracks which pane the user last focused and exposes a
// restore callback the host can fire after PanelLayout's onLayoutChanged.
// Reading pane focus directly from Monaco is unreliable across resizes; a
// data-pane-id attribute on the wrapper element is the stable anchor.
export interface FocusRestoreApi {
  remember: (paneId: string) => void;
  restore: () => void;
  current: () => string | null;
}

export function useFocusRestore(): FocusRestoreApi {
  const lastFocused = useRef<string | null>(null);

  useEffect(() => {
    if (typeof window === "undefined") return;
    const onFocusIn = (event: FocusEvent) => {
      const target = event.target as HTMLElement | null;
      if (!target) return;
      const pane = target.closest<HTMLElement>("[data-pane-id]");
      if (pane?.dataset.paneId) {
        lastFocused.current = pane.dataset.paneId;
      }
    };
    document.addEventListener("focusin", onFocusIn);
    return () => document.removeEventListener("focusin", onFocusIn);
  }, []);

  const remember = useCallback((paneId: string) => {
    lastFocused.current = paneId;
  }, []);

  const restore = useCallback(() => {
    if (!lastFocused.current || typeof document === "undefined") return;
    const el = document.querySelector<HTMLElement>(
      `[data-pane-id="${lastFocused.current}"]`,
    );
    if (!el) return;
    const focusable = el.querySelector<HTMLElement>(
      ".monaco-editor textarea, textarea, input, [tabindex='0']",
    );
    focusable?.focus();
  }, []);

  const current = useCallback(() => lastFocused.current, []);

  return { remember, restore, current };
}
