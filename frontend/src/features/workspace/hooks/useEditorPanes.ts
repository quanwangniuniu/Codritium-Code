"use client";

import { useCallback, useMemo } from "react";
import type { ProblemSession } from "@/features/workspace/lib/session-store";
import { applyFileChange, legacyToPanes } from "@/features/workspace/lib/workspace-state";
import type { EditorPaneState } from "@/features/workspace/types";
import { t } from "@/shared/i18n";
import { toast } from "@/shared/lib/toast";
import { useFocusRestore } from "./useFocusRestore";
import { usePaneLimit } from "./usePaneLimit";
import type { SetSessionFn } from "./useProblemSession";

// Split-pane editor state: which files are open in which pane, which pane
// has focus, and every tab / content mutation. Pane layout is persisted in
// the problem session (editorPanes, mirrored to openTabs / activeTab).
export function useEditorPanes(session: ProblemSession | null, setSession: SetSessionFn) {
  const fileContents = session?.fileContents ?? {};
  const originalContents = session?.originalContents ?? {};

  const editorPanes: EditorPaneState[] = useMemo(() => {
    if (session?.editorPanes && session.editorPanes.length > 0) {
      return session.editorPanes;
    }
    return legacyToPanes(session?.openTabs ?? [], session?.activeTab ?? null);
  }, [session?.editorPanes, session?.openTabs, session?.activeTab]);

  const paneLimit = usePaneLimit(editorPanes.length);
  const focusRestore = useFocusRestore();

  const updateEditorPanes = useCallback(
    (next: EditorPaneState[]) => {
      setSession((prev) => ({
        ...prev,
        editorPanes: next,
        openTabs: next[0]?.tabs ?? [],
        activeTab: next[0]?.activeTab ?? null,
      }));
    },
    [setSession],
  );

  const targetPaneId = useCallback(() => {
    const remembered = focusRestore.current();
    if (remembered && editorPanes.find((p) => p.id === remembered)) {
      return remembered;
    }
    return editorPanes[0]?.id ?? "main";
  }, [focusRestore, editorPanes]);

  const selectFile = (name: string) => {
    const paneId = targetPaneId();
    const next = editorPanes.map((p) => {
      if (p.id !== paneId) return p;
      const alreadyOpen = p.tabs.find((tab) => tab.name === name);
      const tabs = alreadyOpen
        ? p.tabs
        : [...p.tabs, { name, dirty: fileContents[name] !== originalContents[name] }];
      return { ...p, activeTab: name, tabs };
    });
    updateEditorPanes(next);
    focusRestore.remember(paneId);
  };

  const activateTab = (paneId: string, name: string) => {
    updateEditorPanes(editorPanes.map((p) => (p.id === paneId ? { ...p, activeTab: name } : p)));
    focusRestore.remember(paneId);
  };

  const closeTab = (paneId: string, name: string) => {
    updateEditorPanes(
      editorPanes.map((p) => {
        if (p.id !== paneId) return p;
        const remaining = p.tabs.filter((tab) => tab.name !== name);
        const nextActive =
          p.activeTab === name
            ? remaining.length > 0
              ? remaining[remaining.length - 1].name
              : null
            : p.activeTab;
        return { ...p, tabs: remaining, activeTab: nextActive };
      }),
    );
  };

  const changeContent = (_paneId: string, name: string, value: string) => {
    setSession((prev) => applyFileChange(prev, name, value));
  };

  const splitFrom = (fromPaneId: string) => {
    if (!paneLimit.canAddPane) return;
    const fromPane = editorPanes.find((p) => p.id === fromPaneId);
    const newPane: EditorPaneState = {
      id: `pane-${Date.now()}`,
      tabs: fromPane ? fromPane.tabs.map((tab) => ({ ...tab })) : [],
      activeTab: fromPane?.activeTab ?? null,
    };
    const idx = editorPanes.findIndex((p) => p.id === fromPaneId);
    const next = [...editorPanes];
    next.splice(idx + 1, 0, newPane);
    updateEditorPanes(next);
    focusRestore.remember(newPane.id);
  };

  const closePane = (paneId: string) => {
    if (editorPanes.length <= 1) return;
    updateEditorPanes(editorPanes.filter((p) => p.id !== paneId));
  };

  // "Apply" on a chat code block: replace the focused pane's active file.
  const applyToActiveTab = (codeBlock: string) => {
    const paneId = targetPaneId();
    const target = editorPanes.find((p) => p.id === paneId)?.activeTab ?? null;
    if (!target) {
      toast.warn(t("open_a_file_first"));
      return;
    }
    setSession((prev) => applyFileChange(prev, target, codeBlock, true));
  };

  const focusedActiveFile =
    editorPanes.find((p) => p.id === (focusRestore.current() ?? editorPanes[0]?.id))?.activeTab ??
    editorPanes[0]?.activeTab ??
    null;

  return {
    editorPanes,
    canSplit: paneLimit.canAddPane,
    focusedActiveFile,
    selectFile,
    activateTab,
    closeTab,
    changeContent,
    splitFrom,
    closePane,
    applyToActiveTab,
    restoreFocus: focusRestore.restore,
  };
}
