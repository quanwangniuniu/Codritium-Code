"use client";

import { useState, useMemo } from "react";
import { AnimatePresence, motion } from "framer-motion";
import { BookOpen, ChevronDown, ChevronRight } from "lucide-react";
import { FileTree } from "./FileTree";
import { DiffViewer } from "./DiffViewer";
import { ExplanationPanel } from "./ExplanationPanel";
import { t } from "@/lib/i18n";
import { useLocale } from "@/lib/i18n-client";

// AnswerModeView is the static "show me the answer" surface. There is no
// replay cursor and no envelope timeline; the candidate sees the final
// diff for every touched file alongside the standard explanation. Useful
// when the candidate has already failed once and wants the gist without
// stepping through the AI conversation.

export interface AnswerModeViewProps {
  starterFiles: Record<string, string>;
  finalFiles: Record<string, string>;
  explanationMd: string;
}

export function AnswerModeView({
  starterFiles,
  finalFiles,
  explanationMd,
}: AnswerModeViewProps) {
  useLocale();
  // Show all files the engineer touched, plus any starter files for
  // context. Modified files (those whose final differs from starter)
  // bubble to the top of the list.
  const allFiles = useMemo(() => {
    const set = new Set<string>([
      ...Object.keys(starterFiles),
      ...Object.keys(finalFiles),
    ]);
    const arr = Array.from(set);
    arr.sort((a, b) => {
      const aMod = isModified(a, starterFiles, finalFiles);
      const bMod = isModified(b, starterFiles, finalFiles);
      if (aMod && !bMod) return -1;
      if (!aMod && bMod) return 1;
      return a.localeCompare(b);
    });
    return arr;
  }, [starterFiles, finalFiles]);

  const modifiedSet = useMemo(() => {
    return new Set(allFiles.filter((f) => isModified(f, starterFiles, finalFiles)));
  }, [allFiles, starterFiles, finalFiles]);

  const firstModified = allFiles.find((f) => modifiedSet.has(f)) ?? allFiles[0] ?? null;
  const [activeFile, setActiveFile] = useState<string | null>(firstModified);
  const [solutionOpen, setSolutionOpen] = useState(false);

  const starter = activeFile ? starterFiles[activeFile] ?? "" : "";
  const current = activeFile ? finalFiles[activeFile] ?? starter : "";

  return (
    <div
      style={{
        border: "1px solid var(--divider)",
        borderRadius: 10,
        overflow: "hidden",
        background: "var(--surface)",
        display: "flex",
        flexDirection: "column",
      }}
    >
      <div style={{ borderBottom: solutionOpen ? "1px solid var(--divider)" : "none" }}>
        <button
          type="button"
          onClick={() => setSolutionOpen((v) => !v)}
          aria-expanded={solutionOpen}
          style={{
            width: "100%",
            display: "flex",
            alignItems: "center",
            gap: 8,
            padding: "10px 14px",
            background: "var(--surface-2, #f6f7f8)",
            border: "none",
            borderBottom: solutionOpen ? "1px solid var(--divider)" : "1px solid transparent",
            cursor: "pointer",
            fontSize: 11.5,
            fontWeight: 600,
            letterSpacing: 0.5,
            textTransform: "uppercase",
            color: "var(--muted)",
            textAlign: "left",
          }}
        >
          <BookOpen size={13} strokeWidth={2} />
          {t("solution_explanation")}
          <span style={{ marginLeft: "auto", display: "inline-flex", alignItems: "center" }}>
            {solutionOpen ? <ChevronDown size={14} strokeWidth={2} /> : <ChevronRight size={14} strokeWidth={2} />}
          </span>
        </button>
        <AnimatePresence initial={false}>
          {solutionOpen && (
            <motion.div
              key="solution-body"
              initial={{ height: 0, opacity: 0 }}
              animate={{ height: "auto", opacity: 1 }}
              exit={{ height: 0, opacity: 0 }}
              transition={{ duration: 0.24, ease: "easeOut" }}
              style={{ overflow: "hidden" }}
            >
              <div
                className="reply-scroll"
                style={{ maxHeight: 360, overflow: "auto" }}
              >
                <ExplanationPanel markdown={explanationMd} />
              </div>
            </motion.div>
          )}
        </AnimatePresence>
      </div>
      <div
        style={{
          display: "grid",
          gridTemplateColumns: "180px minmax(0, 1fr)",
          gap: 0,
        }}
      >
        <aside
          style={{
            borderRight: "1px solid var(--divider)",
            background: "var(--surface-2, #f6f7f8)",
          }}
        >
          <FileTree
            files={allFiles}
            activeFile={activeFile}
            onSelect={setActiveFile}
            modifiedFiles={modifiedSet}
          />
        </aside>
        <div
          className="reply-scroll reply-diff-wrap"
          style={{
            display: "flex",
            flexDirection: "column",
            minWidth: 0,
            overflow: "auto",
          }}
        >
          {activeFile ? (
            <DiffViewer
              filename={activeFile}
              starter={starter}
              current={current}
              cursorKey={0}
              emptyHint={t("diff_no_content")}
            />
          ) : (
            <div
              style={{
                padding: "32px 16px",
                textAlign: "center",
                color: "var(--muted)",
                fontSize: 12.5,
              }}
            >
              {t("diff_select_from_left")}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

function isModified(
  name: string,
  starter: Record<string, string>,
  finalFiles: Record<string, string>,
): boolean {
  const s = starter[name];
  const f = finalFiles[name];
  if (f === undefined) return false;
  return s !== f;
}
