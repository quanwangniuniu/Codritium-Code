import type { DecisionKind } from "./api";

// One open editor tab in the IDE workspace. `dirty` flips when the buffer
// drifts from the last-persisted starter file content.
export type OpenTab = { name: string; dirty: boolean };

// One editor pane (split view) and its tabs.
export interface EditorPaneState {
  id: string;
  tabs: OpenTab[];
  activeTab: string | null;
}

// PendingPatch is the shape SideBar / PendingPatchesPanel hand to the
// component. It is derived from a backend `tool_use_proposed` event +
// the current Workspace file content (so the diff knows what "before"
// is). For non-FileEdit tools (FileRead / RunTests) we still render an
// approve/reject prompt but no diff.
export interface PendingPatch {
  // Session the tool call belongs to; the backend checks the caller owns it.
  sessionId: string;
  toolUseId: string;
  tool: string; // "FileEdit" | "FileRead" | "RunTests" | ...
  inputSummary: string;
  // For FileEdit: path + content already extracted from the backend
  // event (when the front-end can read it). Non-FileEdit pendings leave
  // path/content empty.
  path?: string;
  newContent?: string;
  oldContent?: string;
  // Auto = true means the runtime auto-executed this tool (read-class).
  // PatchPreview is skipped; ChatPanel renders a compact info line instead.
  auto?: boolean;
}

// onResolved signature carries the effective content that landed in the
// workspace so the caller (workspace page) can sync its local files map
// without re-querying the backend. content is undefined on reject.
export type ResolveCallback = (
  toolUseId: string,
  kind: DecisionKind,
  result: { path?: string; content?: string } | null,
) => void;

export type TextMessage = {
  // kind is optional for backwards compatibility with sessions persisted
  // before patch messages were introduced. Treat missing kind as "text".
  kind?: "text";
  id: string;
  role: "user" | "assistant";
  content: string;
  streaming?: boolean;
};

export type PatchMessage = {
  kind: "patch";
  id: string;
  pending: PendingPatch;
  resolved?: { kind: "approve" | "modify" | "reject" };
};

export type ChatMessage = TextMessage | PatchMessage;

// One structured event from GET /api/sessions/{id}/stream ("agent_event").
export interface StreamEnvelope {
  session_id: string;
  seq: number;
  kind: string;
  emitted_at: string;
  payload: Record<string, unknown>;
}
