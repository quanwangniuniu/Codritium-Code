"use client";

import { useState } from "react";
import { ChevronDown, ChevronRight, GitPullRequest } from "lucide-react";
import { PatchPreview } from "./PatchPreview";
import type { PendingPatch, ResolveCallback } from "../types";
import { t } from "@/shared/i18n";
import { useLocale } from "@/shared/i18n/client";

// PendingPatchesPanel is the SideBar overflow surface. The primary patch
// decision UI lives inline in ChatPanel (D-01). This panel renders only when
// pending.length > 0, collapsed by default to a 48px badge; clicking the
// badge expands the full list in place.
export function PendingPatchesPanel({
  pending,
  onResolved,
}: {
  pending: PendingPatch[];
  onResolved: ResolveCallback;
}) {
  useLocale();
  const [expanded, setExpanded] = useState(false);

  if (pending.length === 0) return null;

  return (
    <div style={{ flexShrink: 0, borderTop: "1px solid var(--border-soft)" }}>
      <button
        onClick={() => setExpanded((v) => !v)}
        style={{
          width: "100%",
          height: 48,
          padding: "0 14px",
          display: "flex",
          alignItems: "center",
          gap: 8,
          background: "transparent",
          border: "none",
          cursor: "pointer",
          color: "var(--text-dim)",
          fontSize: 11,
          textTransform: "uppercase",
          letterSpacing: "0.06em",
          fontWeight: 600,
        }}
        aria-expanded={expanded}
      >
        <GitPullRequest size={13} strokeWidth={1.7} />
        <span>{t("pending_label")}</span>
        <span
          style={{
            background: "var(--accent)",
            color: "#fff",
            borderRadius: 9,
            padding: "1px 7px",
            fontSize: 10,
            fontWeight: 700,
            letterSpacing: 0,
          }}
        >
          {pending.length}
        </span>
        <span style={{ marginLeft: "auto", display: "flex" }}>
          {expanded ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
        </span>
      </button>
      {expanded && (
        <div style={{ padding: "4px 10px 10px", maxHeight: 360, overflowY: "auto" }}>
          {pending.map((p) => (
            <PatchPreview key={p.toolUseId} pending={p} onResolved={onResolved} />
          ))}
        </div>
      )}
    </div>
  );
}
