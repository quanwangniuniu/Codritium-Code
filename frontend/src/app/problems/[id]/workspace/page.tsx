"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useParams, useSearchParams } from "next/navigation";
import { Backend, type ProblemDetail, type DecisionKind } from "@/lib/api";
import { AppBar } from "@/components/AppBar";
import { ActivityBar, type ActivityView } from "@/components/ActivityBar";
import { SideBar } from "@/components/SideBar";
import { ChatPanel, type ChatMessage } from "@/components/ChatPanel";
import { RightPanel } from "@/components/RightPanel";
import { StatusBar } from "@/components/StatusBar";
import { Splitter } from "@/components/Splitter";
import { PendingPatchesPanel } from "@/components/PendingPatchesPanel";
import { EditorAreaSplit, type EditorPaneState } from "@/components/ide/EditorAreaSplit";
import { FloatingPanel } from "@/components/FloatingPanel";
import { ReadmeBody } from "@/components/ReadmeTab";
import { useFocusRestore } from "@/hooks/useFocusRestore";
import { usePaneLimit } from "@/hooks/usePaneLimit";
import type { PendingPatch } from "@/components/PatchPreview";
import type { OpenTab } from "@/lib/types";
import { toast } from "@/lib/toast";
import { t as tr } from "@/lib/i18n";
import { useProblemSession } from "@/hooks/useProblemSession";
import { useSessionStream, type StreamEnvelope } from "@/hooks/useSessionStream";
import { clearRestoredFlag, deleteSession, getSession } from "@/lib/problem-session-store";
import { useRequireAuth } from "@/hooks/useRequireAuth";

void ChatPanel;

function langFor(filename: string): string {
  if (filename.endsWith(".py")) return "python";
  if (filename.endsWith(".ts") || filename.endsWith(".tsx")) return "typescript";
  if (filename.endsWith(".js") || filename.endsWith(".jsx")) return "javascript";
  if (filename.endsWith(".go")) return "go";
  if (filename.endsWith(".md")) return "markdown";
  return "plaintext";
}

function legacyToPanes(openTabs: OpenTab[], activeTab: string | null): EditorPaneState[] {
  return [{ id: "main", tabs: openTabs, activeTab }];
}

export default function ProblemWorkspacePage() {
  const params = useParams<{ id: string }>();
  const search = useSearchParams();
  const slug = params.id;
  const forceFresh = search.get("fresh") === "1";

  const me = useRequireAuth();
  const [problem, setProblem] = useState<ProblemDetail | null>(null);
  const handle = me?.handle ?? null;
  const { session, setSession } = useProblemSession(handle, slug);

  const [activity, setActivity] = useState<ActivityView>("files");
  const [chatBusy, setChatBusy] = useState(false);
  const [readmeFloating, setReadmeFloating] = useState(false);
  const [elapsed, setElapsed] = useState(0);
  const [sideWidth, setSideWidth] = useState(280);
  const [chatWidth, setChatWidth] = useState(380);
  const handleSideResize = useCallback((dx: number) => {
    setSideWidth((w) => Math.max(200, Math.min(560, w + dx)));
  }, []);
  const handleChatResize = useCallback((dx: number) => {
    setChatWidth((w) => Math.max(280, Math.min(640, w - dx)));
  }, []);

  const [sessionId, setSessionId] = useState<string | null>(null);
  const [streamingAssistantId, setStreamingAssistantId] = useState<string | null>(null);
  const [pending, setPending] = useState<PendingPatch[]>([]);

  useEffect(() => {
    if (!handle || !slug) return;
    let cancelled = false;
    Backend.getProblem(slug)
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
        return Backend.createSession(slug, forceFresh);
      })
      .then((s) => {
        if (cancelled || !s) return;
        setSessionId(s.session_id);
      })
      .catch((e) => {
        console.error("workspace bootstrap:", e);
        toast.error(tr("submit_failed"));
      });
    return () => {
      cancelled = true;
    };
  }, [slug, handle, setSession, forceFresh]);

  useEffect(() => {
    if (!session) return;
    const startedAt = session.startedAt;
    const id = setInterval(() => {
      setElapsed(Math.floor((Date.now() - startedAt) / 1000));
    }, 1000);
    return () => clearInterval(id);
  }, [session?.startedAt]);

  useEffect(() => {
    if (!session?.restoredFromStorage || !handle) return;
    toast.info(tr("restored_from_session"));
    clearRestoredFlag(handle, slug);
  }, [session?.restoredFromStorage, handle, slug]);

  const fileContents = session?.fileContents ?? {};
  const originalContents = session?.originalContents ?? {};
  const messages = session?.messages ?? [];
  const fileNames = Object.keys(fileContents);

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

  // ── SSE: envelope → state mutations ───────────────────────────────────
  const onEnvelope = useCallback(
    (env: StreamEnvelope) => {
      switch (env.kind) {
        case "tool_use_proposed": {
          const tool = String(env.payload.tool ?? "");
          const toolUseId = String(env.payload.tool_use_id ?? "");
          const summary = String(env.payload.input_summary ?? "");
          const auto = env.payload.auto === true;
          const path = parseEditPath(summary) ?? parseReadPath(summary);
          const patch: PendingPatch = {
            sessionId: env.session_id,
            toolUseId,
            tool,
            inputSummary: summary,
            path: path ?? undefined,
            oldContent: path ? fileContents[path] : undefined,
            newContent: path ? fileContents[path] : undefined,
            auto,
          };
          if (!auto) {
            setPending((prev) => [...prev, patch]);
          }
          setSession((prev) => ({
            ...prev,
            messages: [
              ...prev.messages,
              {
                kind: "patch",
                id: `p-${toolUseId}`,
                pending: patch,
                ...(auto ? { resolved: { kind: "approve" as const } } : {}),
              },
            ],
          }));
          break;
        }
        case "candidate_approved":
        case "candidate_rejected": {
          const id = String(env.payload.tool_use_id ?? "");
          const decisionKind: "approve" | "reject" =
            env.kind === "candidate_approved" ? "approve" : "reject";
          setPending((prev) => prev.filter((p) => p.toolUseId !== id));
          setSession((prev) => ({
            ...prev,
            messages: prev.messages.map((m) =>
              m.kind === "patch" && m.pending.toolUseId === id
                ? { ...m, resolved: { kind: decisionKind } }
                : m,
            ),
          }));
          break;
        }
        case "turn_completed": {
          if (streamingAssistantId) {
            setSession((prev) => ({
              ...prev,
              messages: prev.messages.map((m) =>
                m.kind !== "patch" && m.id === streamingAssistantId
                  ? { ...m, streaming: false }
                  : m,
              ),
            }));
            setStreamingAssistantId(null);
          }
          setChatBusy(false);
          break;
        }
      }
    },
    [fileContents, streamingAssistantId, setSession],
  );

  const onTextDelta = useCallback(
    (delta: string) => {
      if (!streamingAssistantId) return;
      setSession((prev) => ({
        ...prev,
        messages: prev.messages.map((m) =>
          m.kind !== "patch" && m.id === streamingAssistantId
            ? { ...m, content: m.content + delta }
            : m,
        ),
      }));
    },
    [streamingAssistantId, setSession],
  );

  const streamOpts = useMemo(
    () => ({ onEnvelope, onTextDelta }),
    [onEnvelope, onTextDelta],
  );
  useSessionStream(sessionId, streamOpts);

  // ── Tab + pane handlers ───────────────────────────────────────────────
  const targetPaneId = useCallback(() => {
    const remembered = focusRestore.current();
    if (remembered && editorPanes.find((p) => p.id === remembered)) {
      return remembered;
    }
    return editorPanes[0]?.id ?? "main";
  }, [focusRestore, editorPanes]);

  const handleSelectFile = (name: string) => {
    const paneId = targetPaneId();
    const next = editorPanes.map((p) => {
      if (p.id !== paneId) return p;
      const alreadyOpen = p.tabs.find((tab) => tab.name === name);
      const tabs = alreadyOpen
        ? p.tabs
        : [
          ...p.tabs,
          {
            name,
            dirty: fileContents[name] !== originalContents[name],
          },
        ];
      return { ...p, activeTab: name, tabs };
    });
    updateEditorPanes(next);
    focusRestore.remember(paneId);
  };

  const handleActivateTab = (paneId: string, name: string) => {
    updateEditorPanes(
      editorPanes.map((p) => (p.id === paneId ? { ...p, activeTab: name } : p)),
    );
    focusRestore.remember(paneId);
  };

  const handleCloseTab = (paneId: string, name: string) => {
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

  const handleContentChange = (_paneId: string, name: string, value: string) => {
    setSession((prev) => {
      const newContents = { ...prev.fileContents, [name]: value };
      const basePanes =
        prev.editorPanes && prev.editorPanes.length > 0
          ? prev.editorPanes
          : legacyToPanes(prev.openTabs, prev.activeTab);
      const nextPanes = basePanes.map((pane) => ({
        ...pane,
        tabs: pane.tabs.map((tab) =>
          tab.name === name
            ? { ...tab, dirty: value !== prev.originalContents[name] }
            : tab,
        ),
      }));
      return {
        ...prev,
        fileContents: newContents,
        editorPanes: nextPanes,
        openTabs: nextPanes[0]?.tabs ?? prev.openTabs,
      };
    });
  };

  const handleSplitFromPane = (fromPaneId: string) => {
    if (!paneLimit.canAddPane) return;
    const fromPane = editorPanes.find((p) => p.id === fromPaneId);
    const newPane: EditorPaneState = {
      id: `pane-${Date.now()}`,
      tabs: fromPane ? fromPane.tabs.map((t) => ({ ...t })) : [],
      activeTab: fromPane?.activeTab ?? null,
    };
    const idx = editorPanes.findIndex((p) => p.id === fromPaneId);
    const next = [...editorPanes];
    next.splice(idx + 1, 0, newPane);
    updateEditorPanes(next);
    focusRestore.remember(newPane.id);
  };

  const handleClosePane = (paneId: string) => {
    if (editorPanes.length <= 1) return;
    updateEditorPanes(editorPanes.filter((p) => p.id !== paneId));
  };

  // ── chat send (v0.8: Backend.chat → /api/chat/v2) ─────────────────────
  const handleSend = async (text: string) => {
    if (!sessionId) {
      toast.warn("Session not ready yet. Try again in a moment.");
      return;
    }
    const userMsg: ChatMessage = { id: `u-${Date.now()}`, role: "user", content: text };
    const assistantId = `a-${Date.now()}`;
    const assistantMsg: ChatMessage = { id: assistantId, role: "assistant", content: "", streaming: true };
    setSession((prev) => ({ ...prev, messages: [...prev.messages, userMsg, assistantMsg] }));
    setStreamingAssistantId(assistantId);
    setChatBusy(true);
    try {
      await Backend.chat(sessionId, text, fileContents);
    } catch (e) {
      console.error("chat send failed:", e);
      setSession((prev) => ({
        ...prev,
        messages: prev.messages.filter((m) => m.id !== assistantId),
      }));
      setStreamingAssistantId(null);
      setChatBusy(false);
      toast.error(tr("chat_send_failed"));
    }
  };

  // ── pending patch resolution (PatchPreview → callback) ────────────────
  const handlePatchResolved = useCallback(
    (toolUseId: string, kind: DecisionKind, result: { path?: string; content?: string } | null) => {
      setPending((prev) => prev.filter((p) => p.toolUseId !== toolUseId));
      setSession((prev) => {
        const updatedMessages = prev.messages.map((m) =>
          m.kind === "patch" && m.pending.toolUseId === toolUseId
            ? { ...m, resolved: { kind } }
            : m,
        );
        if (kind !== "reject" && result?.path && result.content !== undefined) {
          const path = result.path;
          const content = result.content;
          const newContents = { ...prev.fileContents, [path]: content };
          const basePanes =
            prev.editorPanes && prev.editorPanes.length > 0
              ? prev.editorPanes
              : legacyToPanes(prev.openTabs, prev.activeTab);
          const nextPanes = basePanes.map((pane) => ({
            ...pane,
            tabs: pane.tabs.map((tab) =>
              tab.name === path
                ? { ...tab, dirty: content !== prev.originalContents[path] }
                : tab,
            ),
          }));
          return {
            ...prev,
            messages: updatedMessages,
            fileContents: newContents,
            editorPanes: nextPanes,
            openTabs: nextPanes[0]?.tabs ?? prev.openTabs,
          };
        }
        return { ...prev, messages: updatedMessages };
      });
    },
    [setSession],
  );

  const handleApply = (codeBlock: string) => {
    const paneId = targetPaneId();
    const target = editorPanes.find((p) => p.id === paneId)?.activeTab ?? null;
    if (!target) {
      toast.warn(tr("open_a_file_first"));
      return;
    }
    setSession((prev) => {
      const newContents = { ...prev.fileContents, [target]: codeBlock };
      const basePanes =
        prev.editorPanes && prev.editorPanes.length > 0
          ? prev.editorPanes
          : legacyToPanes(prev.openTabs, prev.activeTab);
      const nextPanes = basePanes.map((pane) => ({
        ...pane,
        tabs: pane.tabs.map((tab) =>
          tab.name === target ? { ...tab, dirty: true } : tab,
        ),
      }));
      return {
        ...prev,
        fileContents: newContents,
        editorPanes: nextPanes,
        openTabs: nextPanes[0]?.tabs ?? prev.openTabs,
      };
    });
  };

  const handleSubmit = async () => {
    if (!problem) return;
    try {
      const res = await fetch(`${Backend.apiBase}/api/submissions`, {
        method: "POST",
        credentials: "include",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          problem_slug: slug,
          variant: problem.variant,
          code_files: fileContents,
          session_id: sessionId,
        }),
      });
      if (!res.ok) {
        console.error("submit failed:", res.status, await res.text());
        toast.error(tr("submit_failed"));
        return;
      }
      const data = await res.json();
      if (handle) deleteSession(handle, slug);
      window.location.href = `/submissions/${data.id}`;
    } catch (e) {
      console.error("submit error:", e);
      toast.error(tr("submit_failed"));
    }
  };

  const timerStr = `${String(Math.floor(elapsed / 3600)).padStart(2, "0")}:${String(
    Math.floor((elapsed % 3600) / 60),
  ).padStart(2, "0")}:${String(elapsed % 60).padStart(2, "0")}`;

  if (!me) return null;

  void originalContents;

  const focusPaneActive =
    editorPanes.find((p) => p.id === (focusRestore.current() ?? editorPanes[0]?.id))?.activeTab ??
    editorPanes[0]?.activeTab ??
    null;

  return (
    <div style={{ height: "100vh", display: "flex", flexDirection: "column" }}>
      <AppBar
        timer={timerStr}
        onSubmit={handleSubmit}
        problemTitle={problem?.title ?? slug}
      />
      <div style={{ flex: 1, display: "flex", minHeight: 0 }}>
        <ActivityBar active={activity} onChange={setActivity} />
        <div style={{ width: sideWidth, flexShrink: 0, minWidth: 0, display: "flex" }}>
          <SideBar
            files={fileNames.map((n) => ({ name: n, type: "file" as const }))}
            activeFile={focusPaneActive}
            onSelectFile={handleSelectFile}
            extraSlot={
              <PendingPatchesPanel
                pending={pending}
                onResolved={handlePatchResolved}
              />
            }
          />
        </div>
        <Splitter onResize={handleSideResize} />
        <div
          style={{
            flex: 1,
            display: "flex",
            minWidth: 0,
            minHeight: 0,
            position: "relative",
          }}
        >
          <EditorAreaSplit
            panes={editorPanes}
            fileContents={fileContents}
            readmeMD={problem?.readme_md ?? ""}
            readmeFloating={readmeFloating}
            onToggleReadmeFloat={() => setReadmeFloating((v) => !v)}
            onActivateTab={handleActivateTab}
            onCloseTab={handleCloseTab}
            onContentChange={handleContentChange}
            onSplitFrom={handleSplitFromPane}
            onClosePane={handleClosePane}
            canSplit={paneLimit.canAddPane}
            onLayoutChanged={() => focusRestore.restore()}
          />
        </div>
        <Splitter onResize={handleChatResize} />
        <div style={{ width: chatWidth, flexShrink: 0, minWidth: 0, display: "flex" }}>
          <RightPanel
            sessionId={sessionId}
            submitted={false}
            fileContents={fileContents}
            fileNames={fileNames}
            messages={messages}
            onSend={handleSend}
            busy={chatBusy}
            onApply={handleApply}
            onPatchResolved={handlePatchResolved}
          />
        </div>
      </div>
      <StatusBar
        me={me}
        problemSlug={slug}
        language={focusPaneActive ? langFor(focusPaneActive) : "—"}
        cursorLine={1}
        cursorCol={1}
      />
      {readmeFloating && (
        <FloatingPanel
          title="README.md"
          onClose={() => setReadmeFloating(false)}
        >
          <div style={{ padding: "24px 28px" }}>
            <ReadmeBody readme={problem?.readme_md ?? ""} />
          </div>
        </FloatingPanel>
      )}
    </div>
  );
}

function parseEditPath(summary: string): string | null {
  const m = summary.match(/^Edit\s+(\S+)/);
  return m ? m[1] : null;
}

function parseReadPath(summary: string): string | null {
  const m = summary.match(/^Read\s+(\S+)/);
  return m ? m[1] : null;
}
