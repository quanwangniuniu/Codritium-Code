"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import dynamic from "next/dynamic";
import Editor from "@monaco-editor/react";
import {
  AlertTriangle,
  Check,
  FileSearch,
  Pencil,
  Save,
  Search,
  Terminal,
  X,
} from "lucide-react";
import { Backend, type DecisionKind } from "@/lib/api";
import { unifiedDiff, diffStats, type DiffLine } from "@/lib/diff";
import { toast } from "@/lib/toast";
import { t } from "@/lib/i18n";
import { useLocale } from "@/lib/i18n-client";

const DiffEditor = dynamic(
  () => import("@monaco-editor/react").then((mod) => mod.DiffEditor),
  { ssr: false, loading: () => <DiffEditorSkeleton /> },
);

function DiffEditorSkeleton() {
  return (
    <div
      style={{
        padding: "12px 14px",
        fontSize: 11,
        color: "var(--text-muted)",
        background: "var(--bg-editor)",
      }}
    >
      Loading diff…
    </div>
  );
}

function langForPath(path: string | undefined): string {
  if (!path) return "plaintext";
  if (path.endsWith(".py")) return "python";
  if (path.endsWith(".ts") || path.endsWith(".tsx")) return "typescript";
  if (path.endsWith(".js") || path.endsWith(".jsx")) return "javascript";
  if (path.endsWith(".go")) return "go";
  if (path.endsWith(".md")) return "markdown";
  return "plaintext";
}

// PendingPatch is the shape SideBar / PendingPatchesPanel hand to the
// component. It is derived from a backend `tool_use_proposed` event +
// the current Workspace file content (so the diff knows what "before"
// is). For non-FileEdit tools (FileRead / RunTests) we still render an
// approve/reject prompt but no diff.
export interface PendingPatch {
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

export function PatchPreview({
  pending,
  onResolved,
}: {
  pending: PendingPatch;
  onResolved: ResolveCallback;
}) {
  useLocale();
  const [mode, setMode] = useState<"view" | "modify">("view");
  const [modifiedContent, setModifiedContent] = useState<string>(pending.newContent ?? "");
  const [reason, setReason] = useState("");
  const [busy, setBusy] = useState(false);

  const diff: DiffLine[] = useMemo(() => {
    if (pending.tool !== "FileEdit") return [];
    return unifiedDiff(pending.oldContent ?? "", pending.newContent ?? "");
  }, [pending.tool, pending.oldContent, pending.newContent]);
  const stats = diffStats(diff);

  const submit = async (
    kind: DecisionKind,
    opts: { modifiedInput?: string; reason?: string },
    result: { path?: string; content?: string } | null,
  ) => {
    if (busy) return;
    setBusy(true);
    try {
      await Backend.decision(pending.toolUseId, kind, opts);
      onResolved(pending.toolUseId, kind, result);
    } catch (e) {
      console.error("decision failed:", e);
      toast.error(t("patch_decision_failed"));
    } finally {
      setBusy(false);
    }
  };

  const onApprove = () =>
    submit("approve", {}, { path: pending.path, content: pending.newContent });
  const onReject = () => submit("reject", { reason: reason.trim() }, null);
  const onSaveModify = () => {
    const modified = JSON.stringify({ path: pending.path, content: modifiedContent });
    submit("modify", { modifiedInput: modified }, { path: pending.path, content: modifiedContent });
  };

  return (
    <div
      style={{
        border: "1px solid var(--border)",
        borderRadius: 4,
        background: "var(--bg-side)",
        marginBottom: 8,
        overflow: "hidden",
      }}
    >
      <header
        style={{
          padding: "6px 10px",
          borderBottom: "1px solid var(--border)",
          fontSize: 11,
          color: "var(--text-dim)",
          display: "flex",
          alignItems: "center",
          gap: 8,
          background:
            pending.tool === "RunCommand" ? "rgba(244, 162, 89, 0.10)" : undefined,
        }}
      >
        <ToolIcon tool={pending.tool} />
        <span style={{ fontWeight: 600, color: "var(--text-strong)" }}>{pending.tool}</span>
        {pending.path && <span className="mono">{pending.path}</span>}
        {pending.tool === "FileEdit" && (
          <span style={{ marginLeft: "auto", fontSize: 10 }}>
            <span style={{ color: "var(--good)" }}>+{stats.added}</span>{" "}
            <span style={{ color: "var(--bad)" }}>-{stats.removed}</span>
          </span>
        )}
      </header>

      {mode === "view" ? (
        <ToolBody pending={pending} />
      ) : (
        <div style={{ height: 240 }}>
          <Editor
            height="100%"
            language={langForPath(pending.path)}
            theme="vs-dark"
            value={modifiedContent}
            onChange={(v) => setModifiedContent(v ?? "")}
            options={{ minimap: { enabled: false }, fontSize: 12, scrollBeyondLastLine: false }}
          />
        </div>
      )}

      {mode === "view" ? (
        <ActionBar
          busy={busy}
          canModify={pending.tool === "FileEdit"}
          onApprove={onApprove}
          onModify={() => setMode("modify")}
          onReject={onReject}
          reason={reason}
          setReason={setReason}
        />
      ) : (
        <ModifyActionBar
          busy={busy}
          onSave={onSaveModify}
          onCancel={() => {
            setModifiedContent(pending.newContent ?? "");
            setMode("view");
          }}
        />
      )}
    </div>
  );
}

function ToolIcon({ tool }: { tool: string }) {
  switch (tool) {
    case "Grep":
      return <Search size={12} strokeWidth={1.7} style={{ color: "var(--text-dim)" }} />;
    case "Glob":
      return <FileSearch size={12} strokeWidth={1.7} style={{ color: "var(--text-dim)" }} />;
    case "RunCommand":
      return <Terminal size={12} strokeWidth={1.7} style={{ color: "var(--warn)" }} />;
    default:
      return null;
  }
}

function ToolBody({ pending }: { pending: PendingPatch }) {
  switch (pending.tool) {
    case "FileEdit":
      return (
        <div style={{ height: 240, background: "var(--bg-editor)" }}>
          <DiffEditor
            height="100%"
            width="100%"
            language={langForPath(pending.path)}
            original={pending.oldContent ?? ""}
            modified={pending.newContent ?? ""}
            theme="vs-dark"
            options={{
              automaticLayout: true,
              readOnly: true,
              renderSideBySide: false,
              renderOverviewRuler: true,
              minimap: { enabled: false },
              scrollBeyondLastLine: false,
              fontSize: 12,
            }}
          />
        </div>
      );
    case "Grep":
    case "Glob":
      return (
        <div
          style={{
            padding: "10px 12px",
            fontSize: 12,
            color: "var(--text)",
            fontFamily:
              "ui-monospace, SF Mono, Menlo, monospace",
            background: "var(--bg-editor)",
          }}
        >
          {pending.inputSummary}
        </div>
      );
    case "RunCommand":
      return (
        <div
          style={{
            padding: "10px 12px",
            fontSize: 12,
            color: "var(--text)",
            background: "rgba(244, 162, 89, 0.07)",
            borderBottom: "1px solid rgba(244, 162, 89, 0.20)",
          }}
        >
          <div
            style={{
              display: "flex",
              alignItems: "center",
              gap: 6,
              marginBottom: 6,
              fontSize: 10,
              color: "var(--warn)",
            }}
          >
            <AlertTriangle size={11} strokeWidth={1.7} />
            <span style={{ textTransform: "uppercase", letterSpacing: "0.06em" }}>
              {t("patch_runcommand_warn")}
            </span>
          </div>
          <code
            className="mono"
            style={{
              display: "block",
              padding: "6px 8px",
              background: "var(--bg-editor)",
              borderRadius: 3,
              fontSize: 11.5,
              color: "var(--text-strong)",
              wordBreak: "break-all",
            }}
          >
            {pending.inputSummary}
          </code>
        </div>
      );
    default:
      return (
        <div style={{ padding: "10px 12px", fontSize: 12, color: "var(--text)" }}>
          {pending.inputSummary}
        </div>
      );
  }
}

function ActionBar({
  busy,
  canModify,
  onApprove,
  onModify,
  onReject,
  reason,
  setReason,
}: {
  busy: boolean;
  canModify: boolean;
  onApprove: () => void;
  onModify: () => void;
  onReject: () => void;
  reason: string;
  setReason: (s: string) => void;
}) {
  const [rejectArmed, setRejectArmed] = useState(false);
  const reasonRef = useRef<HTMLTextAreaElement>(null);

  useEffect(() => {
    if (rejectArmed) reasonRef.current?.focus();
  }, [rejectArmed]);

  const handleRejectClick = () => {
    if (!rejectArmed) {
      setRejectArmed(true);
      return;
    }
    onReject();
  };

  const handleCancelReject = () => {
    setRejectArmed(false);
    setReason("");
  };

  return (
    <div style={{ borderTop: "1px solid var(--border)" }}>
      <div
        style={{
          padding: "8px 10px",
          display: "flex",
          alignItems: "center",
          gap: 6,
          flexWrap: "wrap",
        }}
      >
        <button
          onClick={onApprove}
          disabled={busy || rejectArmed}
          style={btnStyle("var(--accent)", "#fff", busy || rejectArmed)}
        >
          <Check size={12} strokeWidth={2} /> {t("patch_approve")}
        </button>
        {canModify && (
          <button
            onClick={onModify}
            disabled={busy || rejectArmed}
            style={btnStyle("var(--bg-tab)", "var(--text)", busy || rejectArmed)}
          >
            <Pencil size={12} strokeWidth={1.7} /> {t("patch_modify")}
          </button>
        )}
        <span style={{ marginLeft: "auto", display: "inline-flex", gap: 6 }}>
          {rejectArmed && (
            <button
              onClick={handleCancelReject}
              disabled={busy}
              style={btnStyle("transparent", "var(--text-muted)")}
            >
              {t("cancel")}
            </button>
          )}
          <button
            onClick={handleRejectClick}
            disabled={busy}
            style={btnStyle("var(--bg-tab)", "var(--bad)")}
          >
            <X size={12} strokeWidth={1.7} /> {rejectArmed ? t("patch_reject_confirm") : t("patch_reject")}
          </button>
        </span>
      </div>
      {rejectArmed && (
        <div style={{ padding: "0 10px 10px" }}>
          <textarea
            ref={reasonRef}
            value={reason}
            onChange={(e) => setReason(e.target.value)}
            placeholder={t("patch_reject_reason_placeholder")}
            rows={2}
            style={{
              width: "100%",
              background: "var(--bg-editor)",
              border: "1px solid var(--border)",
              color: "var(--text)",
              padding: "5px 7px",
              borderRadius: 3,
              fontSize: 11,
              resize: "vertical",
            }}
          />
        </div>
      )}
    </div>
  );
}

function ModifyActionBar({
  busy,
  onSave,
  onCancel,
}: {
  busy: boolean;
  onSave: () => void;
  onCancel: () => void;
}) {
  return (
    <div
      style={{
        padding: "8px 10px",
        borderTop: "1px solid var(--border)",
        display: "flex",
        gap: 6,
      }}
    >
      <button onClick={onSave} disabled={busy} style={btnStyle("var(--accent)", "#fff")}>
        <Save size={12} strokeWidth={2} /> {t("patch_save_approve")}
      </button>
      <button onClick={onCancel} disabled={busy} style={btnStyle("var(--bg-tab)", "var(--text)")}>
        {t("cancel")}
      </button>
    </div>
  );
}

function btnStyle(bg: string, color: string, dimmed = false): React.CSSProperties {
  return {
    background: bg,
    color,
    border: "none",
    padding: "4px 9px",
    fontSize: 11,
    borderRadius: 3,
    cursor: dimmed ? "default" : "pointer",
    display: "inline-flex",
    alignItems: "center",
    gap: 4,
    fontWeight: 600,
    opacity: dimmed ? 0.4 : 1,
  };
}
