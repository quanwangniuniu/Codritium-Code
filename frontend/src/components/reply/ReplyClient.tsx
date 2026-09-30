"use client";

import { useMemo, useState } from "react";
import Link from "next/link";
import { motion, AnimatePresence } from "framer-motion";
import {
  FileText,
  Layers,
  ChevronDown,
  ChevronRight,
  BookOpen,
  ArrowUpRight,
} from "lucide-react";
import { transformForReplay, type ReplyEnvelope } from "@/types/reply";
import { StepController } from "./StepController";
import { FileTree } from "./FileTree";
import { DiffViewer } from "./DiffViewer";
import { ExplanationPanel } from "./ExplanationPanel";
import { AnswerModeView } from "./AnswerModeView";
import { t } from "@/shared/i18n";
import { useLocale } from "@/shared/i18n/client";
import {
  extractPatches,
  applyPatches,
  patchCursorFromEnvelopeSeq,
} from "./diff-engine";

// ReplyClient renders ONE walkthrough track (either the official solution or
// the candidate's own run) as a two-column replay plus an optional answer
// view. The official track and a candidate's run are two separate pages; this
// component never mixes them. Both the left diff and the right envelope
// playback derive from the same `data`, so the columns stay consistent.

export interface ReplyTrackData {
  envelopes: ReplyEnvelope[];
  starterFiles: Record<string, string>;
  // final files + prose: meaningful for the official track (answer mode);
  // left empty for a candidate run.
  files: Record<string, string>;
  explanationMd: string;
}

export interface ReplyClientProps {
  variant: "official" | "mine";
  title: string;
  subtitle: string;
  backHref: string;
  backLabel: string;
  data: ReplyTrackData | null;
  emptyReplay: string;
  emptyAnswer: string;
  // mine variant: a link out to the official solution page.
  secondaryHref?: string;
  secondaryLabel?: string;
}

type Mode = "replay" | "answer";

export function ReplyClient({
  variant,
  title,
  subtitle,
  backHref,
  backLabel,
  data,
  emptyReplay,
  emptyAnswer,
  secondaryHref,
  secondaryLabel,
}: ReplyClientProps) {
  useLocale();
  const [mode, setMode] = useState<Mode>("replay");
  const [cursorIdx, setCursorIdx] = useState(0);
  const [solutionOpen, setSolutionOpen] = useState(true);

  // Answer mode (canonical final files + prose) only applies to the official
  // track. A candidate run is replay-only.
  const hasAnswer =
    variant === "official" &&
    !!data &&
    (Object.keys(data.files).length > 0 || data.explanationMd.trim().length > 0);
  const hasExplanation = variant === "official" && !!data?.explanationMd.trim();

  const activeEnvelopes = useMemo(
    () => transformForReplay(data?.envelopes ?? []),
    [data],
  );

  const patches = useMemo(() => extractPatches(data?.envelopes ?? []), [data]);

  const cursorSeq = useMemo(() => {
    if (activeEnvelopes.length === 0) return 0;
    const clamped = Math.min(cursorIdx, activeEnvelopes.length - 1);
    return activeEnvelopes[clamped]?.seq ?? 0;
  }, [activeEnvelopes, cursorIdx]);

  const patchCursor = useMemo(
    () => patchCursorFromEnvelopeSeq(patches, cursorSeq),
    [patches, cursorSeq],
  );

  const cumulative = useMemo(() => {
    if (!data) {
      return { files: {}, lastTouchedFile: null as string | null, failedSeqs: [] };
    }
    return applyPatches(data.starterFiles, patches, patchCursor);
  }, [data, patches, patchCursor]);

  const fileList = useMemo(() => {
    if (!data) return [] as string[];
    const set = new Set<string>([
      ...Object.keys(data.starterFiles),
      ...Object.keys(data.files),
      ...Object.keys(cumulative.files),
    ]);
    return Array.from(set).sort();
  }, [data, cumulative]);

  const modifiedSet = useMemo(() => {
    if (!data) return new Set<string>();
    const s: Record<string, string> = data.starterFiles;
    const f: Record<string, string> = cumulative.files;
    return new Set(Object.keys(f).filter((name) => f[name] !== (s[name] ?? "")));
  }, [cumulative, data]);

  const replayActiveFile = cumulative.lastTouchedFile;
  const [manualActive, setManualActive] = useState<string | null>(null);
  const activeFile =
    manualActive ??
    replayActiveFile ??
    fileList.find((f) => modifiedSet.has(f)) ??
    fileList[0] ??
    null;

  const handleStepChange = (next: number) => {
    setCursorIdx(next);
    setManualActive(null);
  };

  return (
    <main
      style={{
        maxWidth: 1600,
        margin: "0 auto",
        padding: "24px 28px 64px",
        color: "var(--ink)",
      }}
    >
      <style jsx global>{`
        .reply-scroll {
          scrollbar-width: thin;
          scrollbar-color: transparent transparent;
          transition: scrollbar-color 180ms ease;
        }
        .reply-scroll:hover {
          scrollbar-color: rgba(15, 23, 42, 0.22) transparent;
        }
        .reply-scroll::-webkit-scrollbar {
          width: 6px;
          height: 6px;
        }
        .reply-scroll::-webkit-scrollbar-thumb {
          background: transparent;
          border-radius: 3px;
          transition: background 180ms ease;
        }
        .reply-scroll:hover::-webkit-scrollbar-thumb {
          background: rgba(15, 23, 42, 0.2);
        }
        .reply-scroll::-webkit-scrollbar-track {
          background: transparent;
        }
      `}</style>

      <header
        style={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          gap: 16,
          marginBottom: 16,
          flexWrap: "wrap",
        }}
      >
        <div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
          <h1 style={{ fontSize: 22, fontWeight: 600 }}>{title}</h1>
          <p style={{ fontSize: 12.5, color: "var(--muted)", margin: 0 }}>{subtitle}</p>
        </div>
        <div style={{ display: "flex", alignItems: "center", gap: 14 }}>
          {secondaryHref && secondaryLabel && (
            <Link
              href={secondaryHref}
              className="btn-pill btn-pill--ghost"
              style={{ fontSize: 12.5, padding: "8px 14px" }}
            >
              <ArrowUpRight size={14} strokeWidth={1.9} />
              {secondaryLabel}
            </Link>
          )}
          <Link
            href={backHref}
            style={{ fontSize: 12.5, color: "var(--muted)", textDecoration: "none" }}
          >
            {backLabel}
          </Link>
        </div>
      </header>

      {hasAnswer && (
        <div role="tablist" style={{ display: "flex", gap: 4, marginBottom: 18 }}>
          <ModeTab
            active={mode === "replay"}
            onClick={() => setMode("replay")}
            icon={<Layers size={14} strokeWidth={2} />}
            label={t("reply_mode_tab")}
          />
          <ModeTab
            active={mode === "answer"}
            onClick={() => setMode("answer")}
            icon={<FileText size={14} strokeWidth={2} />}
            label={t("answer_mode_tab")}
          />
        </div>
      )}

      <AnimatePresence mode="wait" initial={false}>
        {hasAnswer && mode === "answer" ? (
          <motion.div
            key="answer"
            initial={{ opacity: 0, y: 8 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: -8 }}
            transition={{ duration: 0.22, ease: "easeOut" }}
          >
            {data ? (
              <AnswerModeView
                starterFiles={data.starterFiles}
                finalFiles={data.files}
                explanationMd={data.explanationMd}
              />
            ) : (
              <EmptyCard>{emptyAnswer}</EmptyCard>
            )}
          </motion.div>
        ) : (
          <motion.div
            key="replay"
            initial={{ opacity: 0, y: 8 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: -8 }}
            transition={{ duration: 0.22, ease: "easeOut" }}
            style={{
              display: "grid",
              gridTemplateColumns: "minmax(0, 1.5fr) minmax(0, 1fr)",
              gap: 16,
            }}
          >
            <section
              style={{
                background: "var(--surface)",
                border: "1px solid var(--divider)",
                borderRadius: 10,
                overflow: "hidden",
                display: "flex",
                flexDirection: "column",
                height: "calc(100vh - 80px)",
                maxHeight: 984,
                position: "sticky",
                top: 16,
                alignSelf: "start",
              }}
            >
              {data ? (
                <>
                  <div
                    style={{
                      display: "grid",
                      gridTemplateColumns: "180px minmax(0, 1fr)",
                      gap: 0,
                      flex: "1 1 auto",
                      minHeight: 0,
                      overflow: "hidden",
                    }}
                  >
                    <div
                      style={{
                        borderRight: "1px solid var(--divider)",
                        background: "var(--surface-2, #f6f7f8)",
                      }}
                    >
                      <FileTree
                        files={fileList}
                        activeFile={activeFile}
                        onSelect={setManualActive}
                        modifiedFiles={modifiedSet}
                      />
                    </div>
                    <div
                      className="reply-scroll reply-diff-wrap"
                      style={{ minWidth: 0, overflow: "auto" }}
                    >
                      {activeFile ? (
                        <DiffViewer
                          filename={activeFile}
                          starter={data.starterFiles[activeFile] ?? ""}
                          current={
                            cumulative.files[activeFile] ??
                            data.starterFiles[activeFile] ??
                            ""
                          }
                          cursorKey={patchCursor}
                        />
                      ) : (
                        <div style={{ padding: 16, color: "var(--muted)", fontSize: 12.5 }}>
                          {t("diff_select_hint")}
                        </div>
                      )}
                    </div>
                  </div>
                  {hasExplanation && (
                    <SolutionAccordion
                      open={solutionOpen}
                      onToggle={() => setSolutionOpen((v) => !v)}
                      markdown={data.explanationMd}
                      position="bottom"
                    />
                  )}
                </>
              ) : (
                <EmptyCard>{emptyReplay}</EmptyCard>
              )}
            </section>

            <section
              style={{
                background: "var(--surface)",
                border: "1px solid var(--divider)",
                borderRadius: 10,
                overflow: "hidden",
                height: "calc(100vh - 80px)",
                maxHeight: 984,
                display: "flex",
                flexDirection: "column",
                position: "sticky",
                top: 16,
                alignSelf: "start",
              }}
            >
              <div
                style={{
                  padding: 12,
                  flex: "1 1 auto",
                  minHeight: 0,
                  display: "flex",
                  flexDirection: "column",
                }}
              >
                <StepController
                  envelopes={activeEnvelopes}
                  idx={Math.min(cursorIdx, Math.max(0, activeEnvelopes.length - 1))}
                  onIdxChange={handleStepChange}
                  emptyHint={emptyReplay}
                />
              </div>
            </section>
          </motion.div>
        )}
      </AnimatePresence>
    </main>
  );
}

function ModeTab({
  active,
  onClick,
  icon,
  label,
}: {
  active: boolean;
  onClick: () => void;
  icon: React.ReactNode;
  label: string;
}) {
  return (
    <button
      type="button"
      role="tab"
      aria-selected={active}
      onClick={onClick}
      style={{
        display: "inline-flex",
        alignItems: "center",
        gap: 6,
        padding: "8px 16px",
        border: "1px solid var(--divider)",
        background: active ? "var(--surface)" : "transparent",
        color: active ? "var(--ink)" : "var(--muted)",
        fontSize: 13,
        fontWeight: active ? 600 : 500,
        borderRadius: 7,
        cursor: "pointer",
      }}
    >
      {icon}
      {label}
    </button>
  );
}

function SolutionAccordion({
  open,
  onToggle,
  markdown,
  position = "top",
}: {
  open: boolean;
  onToggle: () => void;
  markdown: string;
  position?: "top" | "bottom";
}) {
  const atBottom = position === "bottom";
  const button = (
    <button
      type="button"
      onClick={onToggle}
      aria-expanded={open}
      style={{
        width: "100%",
        display: "flex",
        alignItems: "center",
        gap: 8,
        padding: "10px 14px",
        background: "var(--surface-2, #f6f7f8)",
        border: "none",
        ...(atBottom
          ? { borderTop: "1px solid var(--divider)" }
          : { borderBottom: open ? "1px solid var(--divider)" : "1px solid transparent" }),
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
        {open ? (
          <ChevronDown size={14} strokeWidth={2} />
        ) : (
          <ChevronRight size={14} strokeWidth={2} />
        )}
      </span>
    </button>
  );

  const body = (
    <AnimatePresence initial={false}>
      {open && (
        <motion.div
          key="solution-body"
          initial={{ height: 0, opacity: 0 }}
          animate={{ height: "auto", opacity: 1 }}
          exit={{ height: 0, opacity: 0 }}
          transition={{ duration: 0.24, ease: "easeOut" }}
          style={{ overflow: "hidden" }}
        >
          <div className="reply-scroll" style={{ maxHeight: 320, overflow: "auto" }}>
            <ExplanationPanel markdown={markdown} />
          </div>
        </motion.div>
      )}
    </AnimatePresence>
  );

  return (
    <div
      style={{
        flex: "0 0 auto",
        ...(atBottom && {
          borderTop: "2px solid var(--divider, #d4d4d8)",
          background: "var(--surface-2, #f6f7f8)",
        }),
      }}
    >
      {atBottom ? (
        <>
          {body}
          {button}
        </>
      ) : (
        <>
          {button}
          {body}
        </>
      )}
    </div>
  );
}

function EmptyCard({ children }: { children: React.ReactNode }) {
  return (
    <div
      style={{
        background: "var(--surface)",
        border: "1px solid var(--divider)",
        borderRadius: 10,
        padding: "32px 24px",
        textAlign: "center",
        color: "var(--muted)",
        fontSize: 13,
        lineHeight: 1.55,
        margin: 16,
      }}
    >
      {children}
    </div>
  );
}
