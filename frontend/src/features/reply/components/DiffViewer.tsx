"use client";

import { useMemo } from "react";
import { AnimatePresence, motion } from "framer-motion";
import { diffLines } from "diff";
import { t } from "@/shared/i18n";
import { useLocale } from "@/shared/i18n/client";

// DiffViewer renders a single-column unified diff between starter and
// current cumulative state for one file. The layout mirrors Claude Code's
// inline diff: one gutter holding the new-file line number, a thin
// marker column with +/-/space, and the content with a green/red row
// background for added/removed lines. Long code lines overflow
// horizontally - the parent's auto-hide scrollbar handles that.
//
// Custom-rendered instead of react-diff-viewer-continued because the
// latter forces a two-column line-number gutter and table-layout: fixed
// that fights any single-column override.

export interface DiffViewerProps {
  filename: string;
  starter: string;
  current: string;
  cursorKey: number;   // forces fade animation when this changes
  emptyHint?: string;
}

interface DiffRow {
  kind: "add" | "del" | "ctx";
  newLine: number | null;   // line number to display in gutter
  text: string;
}

function buildRows(starter: string, current: string): DiffRow[] {
  // diffLines splits on newlines and tags each block as added/removed/
  // unchanged. We walk it once, expand each block back into rows, and
  // assign line numbers in the NEW file (deleted lines borrow the line
  // number where they would have lived had they not been removed).
  const parts = diffLines(starter, current);
  const rows: DiffRow[] = [];
  let newLine = 0;
  for (const part of parts) {
    const lines = part.value.split("\n");
    if (lines.length > 0 && lines[lines.length - 1] === "") lines.pop();
    for (const line of lines) {
      if (part.added) {
        newLine++;
        rows.push({ kind: "add", newLine, text: line });
      } else if (part.removed) {
        // Removed line takes no slot in the new file; show its gutter
        // as null so the column reads as the new-file line numbering
        // exclusively.
        rows.push({ kind: "del", newLine: null, text: line });
      } else {
        newLine++;
        rows.push({ kind: "ctx", newLine, text: line });
      }
    }
  }
  return rows;
}

const KIND_STYLE: Record<DiffRow["kind"], { bg: string; marker: string; markerColor: string }> = {
  add: {
    bg: "rgba(34, 197, 94, 0.14)",
    marker: "+",
    markerColor: "#16a34a",
  },
  del: {
    bg: "rgba(239, 68, 68, 0.14)",
    marker: "-",
    markerColor: "#dc2626",
  },
  ctx: {
    bg: "transparent",
    marker: " ",
    markerColor: "var(--muted)",
  },
};

export function DiffViewer({
  filename,
  starter,
  current,
  cursorKey,
  emptyHint,
}: DiffViewerProps) {
  useLocale();
  const rows = useMemo(() => buildRows(starter, current), [starter, current]);

  if (rows.length === 0) {
    return (
      <div
        style={{
          padding: "24px 16px",
          textAlign: "center",
          color: "var(--muted)",
          fontSize: 12.5,
        }}
      >
        {emptyHint ?? t("diff_select_hint")}
      </div>
    );
  }

  // The widest line number wins the gutter width so digits line up
  // regardless of file length. Add a small padding so two-digit and
  // three-digit numbers don't visually shift the marker column.
  const maxLineNum = rows.reduce((m, r) => (r.newLine && r.newLine > m ? r.newLine : m), 0);
  const gutterCh = Math.max(2, String(maxLineNum).length);

  return (
    <div className="relative">
      <div
        style={{
          display: "flex",
          alignItems: "center",
          gap: 8,
          padding: "6px 12px",
          borderBottom: "1px solid var(--divider)",
          fontSize: 12,
          fontFamily: "var(--font-jetbrains-mono), ui-monospace, monospace",
          color: "var(--muted)",
          background: "var(--surface-2, #f6f7f8)",
        }}
      >
        {filename}
      </div>
      <AnimatePresence mode="wait" initial={false}>
        <motion.div
          key={cursorKey}
          initial={{ opacity: 0, y: 4 }}
          animate={{ opacity: 1, y: 0 }}
          exit={{ opacity: 0, y: -4 }}
          transition={{ duration: 0.22, ease: "easeOut" }}
          style={{
            fontFamily: "var(--font-jetbrains-mono), ui-monospace, monospace",
            fontSize: 12,
            lineHeight: "20px",
            paddingTop: 4,
            paddingBottom: 4,
          }}
        >
          {rows.map((row, i) => {
            const style = KIND_STYLE[row.kind];
            return (
              <div
                key={i}
                style={{
                  display: "flex",
                  background: style.bg,
                  minHeight: 20,
                }}
              >
                <span
                  style={{
                    flex: "0 0 auto",
                    width: `${gutterCh + 2}ch`,
                    paddingLeft: 8,
                    paddingRight: 8,
                    textAlign: "right",
                    color: "var(--muted)",
                    userSelect: "none",
                    fontVariantNumeric: "tabular-nums",
                  }}
                >
                  {row.newLine ?? ""}
                </span>
                <span
                  style={{
                    flex: "0 0 auto",
                    width: "2ch",
                    textAlign: "center",
                    color: style.markerColor,
                    fontWeight: 600,
                    userSelect: "none",
                  }}
                >
                  {style.marker}
                </span>
                <span
                  style={{
                    flex: "1 1 auto",
                    whiteSpace: "pre",
                    minWidth: 0,
                    paddingRight: 8,
                  }}
                >
                  {row.text || " "}
                </span>
              </div>
            );
          })}
        </motion.div>
      </AnimatePresence>
    </div>
  );
}
