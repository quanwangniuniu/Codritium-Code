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

// Converts a tool proposal into UI state. For FileEdit, it combines the
// current file with old_text/new_text so the editor can preview the full diff.
export function patchFromProposal(
  env: StreamEnvelope,
  fileContents: Record<string, string>,
): PendingPatch {
  const summary = String(env.payload.input_summary ?? "");
  const rawInput = env.payload.input;
  const input =
    rawInput &&
      typeof rawInput === "object" &&
      !Array.isArray(rawInput)
      ? (rawInput as Record<string, unknown>)
      : {};

  const inputPath =
    typeof input.path === "string" && input.path
      ? input.path
      : null;
  const path = inputPath ?? parseToolPath(summary);
  const oldContent = path ? fileContents[path] ?? "" : undefined;

  let newContent = oldContent;

  if (env.payload.tool === "FileEdit" && path) {
    if (typeof input.content === "string") {
      // Backwards compatibility for stored full-file FileEdit calls.
      newContent = input.content;
    } else if (typeof input.new_text === "string") {
      const oldText =
        typeof input.old_text === "string" ? input.old_text : "";

      if (oldText) {
        newContent = (oldContent ?? "").replace(
          oldText,
          input.new_text,
        );
      } else {
        // An empty old_text means that FileEdit is creating a new file.
        newContent = input.new_text;
      }
    }
  }

  return {
    sessionId: env.session_id,
    toolUseId: String(env.payload.tool_use_id ?? ""),
    tool: String(env.payload.tool ?? ""),
    inputSummary: summary,
    path: path ?? undefined,
    oldContent,
    newContent,
    auto: env.payload.auto === true,
  };
}

// Marks the matching tool proposal as resolved. Approved FileEdit content
// is staged on the message and only applied after a successful tool_result.
export function markPatchResolved(
  messages: ChatMessage[],
  toolUseId: string,
  kind: "approve" | "modify" | "reject",
  approvedChange?: { path: string; content: string },
): ChatMessage[] {
  return messages.map((message) =>
    message.kind === "patch" &&
      message.pending.toolUseId === toolUseId
      ? {
        ...message,
        resolved: { kind },
        ...(approvedChange ? { approvedChange } : {}),
      }
      : message,
  );
}

// Attaches the backend execution result to the matching tool card.
export function attachToolResult(
  messages: ChatMessage[],
  toolUseId: string,
  result: {
    summary: string;
    isError: boolean;
    durationMs?: number;
  },
): ChatMessage[] {
  return messages.map((message) =>
    message.kind === "patch" &&
      message.pending.toolUseId === toolUseId
      ? {
        ...message,
        toolResult: result,
      }
      : message,
  );
}

// Records a backend tool result and applies a staged FileEdit only when
// execution succeeded.
export function applyToolResult(
  session: ProblemSession,
  toolUseId: string,
  result: {
    summary: string;
    isError: boolean;
    durationMs?: number;
  },
): ProblemSession {
  const messages = attachToolResult(
    session.messages,
    toolUseId,
    result,
  );

  const patchMessage = messages.find(
    (message) =>
      message.kind === "patch" &&
      message.pending.toolUseId === toolUseId,
  );

  const nextSession = {
    ...session,
    messages,
  };

  if (
    !result.isError &&
    patchMessage?.kind === "patch" &&
    patchMessage.approvedChange
  ) {
    return applyFileChange(
      nextSession,
      patchMessage.approvedChange.path,
      patchMessage.approvedChange.content,
    );
  }

  return nextSession;
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
