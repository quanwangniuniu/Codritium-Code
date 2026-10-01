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
import { workspaceApi, type DecisionKind } from "@/features/workspace/api";
import { unifiedDiff, diffStats, type DiffLine } from "@/shared/lib/diff";
import { toast } from "@/shared/lib/toast";
import { cn } from "@/shared/lib/cn";
import { languageForFile } from "@/shared/editor/language";
import { t } from "@/shared/i18n";
import { useLocale } from "@/shared/i18n/client";
import type { PendingPatch, ResolveCallback } from "../types";

const DiffEditor = dynamic(
  () => import("@monaco-editor/react").then((mod) => mod.DiffEditor),
  { ssr: false, loading: () => <DiffEditorSkeleton /> },
);

function DiffEditorSkeleton() {
  return (
    <div className="bg-ide-editor px-3.5 py-3 text-[11px] text-ide-text-muted">
      Loading diff…
    </div>
  );
}

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
      await workspaceApi.decision(pending.sessionId, pending.toolUseId, kind, opts);
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
    <div className="mb-2 overflow-hidden rounded border border-ide-border bg-ide-side">
      <header
        className={cn(
          "flex items-center gap-2 border-b border-ide-border px-2.5 py-1.5 text-[11px] text-ide-text-dim",
          pending.tool === "RunCommand" && "bg-[rgba(244,162,89,0.10)]",
        )}
      >
        <ToolIcon tool={pending.tool} />
        <span className="font-semibold text-ide-text-strong">{pending.tool}</span>
        {pending.path && <span className="mono">{pending.path}</span>}
        {pending.tool === "FileEdit" && (
          <span className="ml-auto text-[10px]">
            <span className="text-ide-good">+{stats.added}</span>{" "}
            <span className="text-ide-bad">-{stats.removed}</span>
          </span>
        )}
      </header>

      {mode === "view" ? (
        <ToolBody pending={pending} />
      ) : (
        <div className="h-[240px]">
          <Editor
            height="100%"
            language={languageForFile(pending.path)}
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
      return <Search size={12} strokeWidth={1.7} className="text-ide-text-dim" />;
    case "Glob":
      return <FileSearch size={12} strokeWidth={1.7} className="text-ide-text-dim" />;
    case "RunCommand":
      return <Terminal size={12} strokeWidth={1.7} className="text-ide-warn" />;
    default:
      return null;
  }
}

function ToolBody({ pending }: { pending: PendingPatch }) {
  switch (pending.tool) {
    case "FileEdit":
      return (
        <div className="h-[240px] bg-ide-editor">
          <DiffEditor
            height="100%"
            width="100%"
            language={languageForFile(pending.path)}
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
            <span className="uppercase tracking-[0.06em]">
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
        <div className="px-3 py-2.5 text-xs text-ide-text">
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
    <div className="border-t border-ide-border">
      <div className="flex flex-wrap items-center gap-1.5 px-2.5 py-2">
        <button
          onClick={onApprove}
          disabled={busy || rejectArmed}
          className={btnClass("primary", busy || rejectArmed)}
        >
          <Check size={12} strokeWidth={2} /> {t("patch_approve")}
        </button>
        {canModify && (
          <button
            onClick={onModify}
            disabled={busy || rejectArmed}
            className={btnClass("secondary", busy || rejectArmed)}
          >
            <Pencil size={12} strokeWidth={1.7} /> {t("patch_modify")}
          </button>
        )}
        <span className="ml-auto inline-flex gap-1.5">
          {rejectArmed && (
            <button
              onClick={handleCancelReject}
              disabled={busy}
              className={btnClass("ghost")}
            >
              {t("cancel")}
            </button>
          )}
          <button
            onClick={handleRejectClick}
            disabled={busy}
            className={btnClass("danger")}
          >
            <X size={12} strokeWidth={1.7} /> {rejectArmed ? t("patch_reject_confirm") : t("patch_reject")}
          </button>
        </span>
      </div>
      {rejectArmed && (
        <div className="px-2.5 pb-2.5">
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
    <div className="flex gap-1.5 border-t border-ide-border px-2.5 py-2">
      <button onClick={onSave} disabled={busy} className={btnClass("primary")}>
        <Save size={12} strokeWidth={2} /> {t("patch_save_approve")}
      </button>
      <button onClick={onCancel} disabled={busy} className={btnClass("secondary")}>
        {t("cancel")}
      </button>
    </div>
  );
}

type BtnTone = "primary" | "secondary" | "ghost" | "danger";

const BTN_TONE: Record<BtnTone, string> = {
  primary: "bg-ide-accent text-white",
  secondary: "bg-ide-tab text-ide-text",
  ghost: "bg-transparent text-ide-text-muted",
  danger: "bg-ide-tab text-ide-bad",
};

function btnClass(tone: BtnTone, dimmed = false): string {
  return cn(
    "inline-flex items-center gap-1 rounded-[3px] border-none px-[9px] py-1 text-[11px] font-semibold",
    BTN_TONE[tone],
    dimmed ? "cursor-default opacity-40" : "cursor-pointer",
  );
}
