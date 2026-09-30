// Pure state transitions for the workspace. The hooks in ../hooks call
// these inside setSession updaters so the logic is testable without React.

import type { ProblemSession } from "./session-store";
import type {
  ChatMessage,
  EditorPaneState,
  OpenTab,
  PendingPatch,
  StreamEnvelope,
} from "../types";

// Sessions persisted before split panes existed only carry openTabs /
// activeTab; treat them as a single "main" pane.
export function legacyToPanes(openTabs: OpenTab[], activeTab: string | null): EditorPaneState[] {
  return [{ id: "main", tabs: openTabs, activeTab }];
}

export function panesOf(
  s: Pick<ProblemSession, "editorPanes" | "openTabs" | "activeTab">,
): EditorPaneState[] {
  return s.editorPanes && s.editorPanes.length > 0
    ? s.editorPanes
    : legacyToPanes(s.openTabs, s.activeTab);
}

// Writes `content` to `path` and refreshes that file's dirty flag in every
// pane. `dirty` defaults to "differs from the starter snapshot".
export function applyFileChange(
  prev: ProblemSession,
  path: string,
  content: string,
  dirty: boolean = content !== prev.originalContents[path],
): ProblemSession {
  const nextPanes = panesOf(prev).map((pane) => ({
    ...pane,
    tabs: pane.tabs.map((tab) => (tab.name === path ? { ...tab, dirty } : tab)),
  }));
  return {
    ...prev,
    fileContents: { ...prev.fileContents, [path]: content },
    editorPanes: nextPanes,
    openTabs: nextPanes[0]?.tabs ?? prev.openTabs,
  };
}

// "Edit foo.py ..." / "Read foo.py" -> "foo.py".
export function parseToolPath(summary: string): string | null {
  const m = summary.match(/^(?:Edit|Read)\s+(\S+)/);
  return m ? m[1] : null;
}

// tool_use_proposed envelope -> PendingPatch. The diff baseline is the
// current workspace content of the referenced file.
export function patchFromProposal(
  env: StreamEnvelope,
  fileContents: Record<string, string>,
): PendingPatch {
  const summary = String(env.payload.input_summary ?? "");
  const path = parseToolPath(summary);
  return {
    sessionId: env.session_id,
    toolUseId: String(env.payload.tool_use_id ?? ""),
    tool: String(env.payload.tool ?? ""),
    inputSummary: summary,
    path: path ?? undefined,
    oldContent: path ? fileContents[path] : undefined,
    newContent: path ? fileContents[path] : undefined,
    auto: env.payload.auto === true,
  };
}

export function markPatchResolved(
  messages: ChatMessage[],
  toolUseId: string,
  kind: "approve" | "modify" | "reject",
): ChatMessage[] {
  return messages.map((m) =>
    m.kind === "patch" && m.pending.toolUseId === toolUseId ? { ...m, resolved: { kind } } : m,
  );
}

// Applies `fn` to the text message with the given id (patch entries are
// never touched).
export function updateTextMessage(
  messages: ChatMessage[],
  id: string,
  fn: (m: Exclude<ChatMessage, { kind: "patch" }>) => Exclude<ChatMessage, { kind: "patch" }>,
): ChatMessage[] {
  return messages.map((m) => (m.kind !== "patch" && m.id === id ? fn(m) : m));
}
