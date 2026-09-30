"use client";

import { useCallback, useState } from "react";
import { useParams, useSearchParams } from "next/navigation";
import { useRequireAuth } from "@/features/auth/hooks/useRequireAuth";
import { ActivityBar, type ActivityView } from "@/features/workspace/components/ActivityBar";
import { AppBar } from "@/features/workspace/components/AppBar";
import { FloatingPanel } from "@/features/workspace/components/FloatingPanel";
import { EditorAreaSplit } from "@/features/workspace/components/ide/EditorAreaSplit";
import { PendingPatchesPanel } from "@/features/workspace/components/PendingPatchesPanel";
import { ReadmeBody } from "@/features/workspace/components/ReadmeTab";
import { RightPanel } from "@/features/workspace/components/RightPanel";
import { SideBar } from "@/features/workspace/components/SideBar";
import { StatusBar } from "@/features/workspace/components/StatusBar";
import { useEditorPanes } from "@/features/workspace/hooks/useEditorPanes";
import { useElapsedSeconds } from "@/features/workspace/hooks/useElapsedSeconds";
import { usePendingPatches } from "@/features/workspace/hooks/usePendingPatches";
import { useProblemSession } from "@/features/workspace/hooks/useProblemSession";
import { useRestoredSessionToast } from "@/features/workspace/hooks/useRestoredSessionToast";
import { useSessionEvents } from "@/features/workspace/hooks/useSessionEvents";
import { useSubmitSolution } from "@/features/workspace/hooks/useSubmitSolution";
import { useWorkspaceBootstrap } from "@/features/workspace/hooks/useWorkspaceBootstrap";
import { languageForFile } from "@/shared/editor/language";
import { formatDuration } from "@/shared/format";
import { Splitter } from "@/shared/ui/Splitter";

// The problem workspace: file tree + pending patches | split editor |
// agent chat / tutor. State lives in the hooks; this component is layout.
export function WorkspacePage() {
  const params = useParams<{ id: string }>();
  const search = useSearchParams();
  const slug = params.id;
  const forceFresh = search.get("fresh") === "1";

  const me = useRequireAuth();
  const handle = me?.handle ?? null;
  const { session, setSession } = useProblemSession(handle, slug);
  const { problem, sessionId } = useWorkspaceBootstrap(handle, slug, forceFresh, setSession);
  useRestoredSessionToast(session?.restoredFromStorage, handle, slug);
  const elapsed = useElapsedSeconds(session?.startedAt);

  const [activity, setActivity] = useState<ActivityView>("files");
  const [readmeFloating, setReadmeFloating] = useState(false);
  const [sideWidth, setSideWidth] = useState(280);
  const [chatWidth, setChatWidth] = useState(380);
  const handleSideResize = useCallback((dx: number) => {
    setSideWidth((w) => Math.max(200, Math.min(560, w + dx)));
  }, []);
  const handleChatResize = useCallback((dx: number) => {
    setChatWidth((w) => Math.max(280, Math.min(640, w - dx)));
  }, []);

  const fileContents = session?.fileContents ?? {};
  const messages = session?.messages ?? [];
  const fileNames = Object.keys(fileContents);

  const panes = useEditorPanes(session, setSession);
  const patches = usePendingPatches(setSession);
  const chat = useSessionEvents({
    sessionId,
    fileContents,
    setSession,
    onProposal: patches.addPending,
    onDecided: patches.dropPending,
  });
  const submit = useSubmitSolution({ problem, slug, handle, sessionId, fileContents });

  if (!me) return null;

  const activeFile = panes.focusedActiveFile;

  return (
    <div className="flex h-screen flex-col">
      <AppBar timer={formatDuration(elapsed)} onSubmit={submit} problemTitle={problem?.title ?? slug} />
      <div className="flex min-h-0 flex-1">
        <ActivityBar active={activity} onChange={setActivity} />
        <div className="flex min-w-0 shrink-0" style={{ width: sideWidth }}>
          <SideBar
            files={fileNames.map((n) => ({ name: n, type: "file" as const }))}
            activeFile={activeFile}
            onSelectFile={panes.selectFile}
            extraSlot={<PendingPatchesPanel pending={patches.pending} onResolved={patches.resolvePatch} />}
          />
        </div>
        <Splitter onResize={handleSideResize} />
        <div className="relative flex min-h-0 min-w-0 flex-1">
          <EditorAreaSplit
            panes={panes.editorPanes}
            fileContents={fileContents}
            readmeMD={problem?.readme_md ?? ""}
            readmeFloating={readmeFloating}
            onToggleReadmeFloat={() => setReadmeFloating((v) => !v)}
            onActivateTab={panes.activateTab}
            onCloseTab={panes.closeTab}
            onContentChange={panes.changeContent}
            onSplitFrom={panes.splitFrom}
            onClosePane={panes.closePane}
            canSplit={panes.canSplit}
            onLayoutChanged={() => panes.restoreFocus()}
          />
        </div>
        <Splitter onResize={handleChatResize} />
        <div className="flex min-w-0 shrink-0" style={{ width: chatWidth }}>
          <RightPanel
            sessionId={sessionId}
            submitted={false}
            fileContents={fileContents}
            fileNames={fileNames}
            messages={messages}
            onSend={chat.send}
            busy={chat.busy}
            onApply={panes.applyToActiveTab}
            onPatchResolved={patches.resolvePatch}
          />
        </div>
      </div>
      <StatusBar
        me={me}
        problemSlug={slug}
        language={activeFile ? languageForFile(activeFile) : "—"}
        cursorLine={1}
        cursorCol={1}
      />
      {readmeFloating && (
        <FloatingPanel title="README.md" onClose={() => setReadmeFloating(false)}>
          <div className="px-7 py-6">
            <ReadmeBody readme={problem?.readme_md ?? ""} />
          </div>
        </FloatingPanel>
      )}
    </div>
  );
}
