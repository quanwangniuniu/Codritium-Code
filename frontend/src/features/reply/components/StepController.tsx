"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { ChevronLeft, ChevronRight } from "lucide-react";
import type { ReplyEnvelope } from "@/features/reply/types";
import { EnvelopeCard } from "./EnvelopeCard";
import { t } from "@/shared/i18n";
import { useLocale } from "@/shared/i18n/client";

// StepController paces the replay one envelope at a time. The candidate
// drives progression with Next / Prev so the official walkthrough reads
// like an interactive conversation rather than a wall of text. Source-
// agnostic: official and user replays share the same component.
// When `idx` + `onIdxChange` are provided the controller is fully
// controlled by the parent (used by the two-column page to sync the
// left-side file/diff state); otherwise it manages its own cursor.

export function StepController({
  envelopes,
  emptyHint,
  idx: controlledIdx,
  onIdxChange,
}: {
  envelopes: ReplyEnvelope[];
  emptyHint?: string;
  idx?: number;
  onIdxChange?: (next: number) => void;
}) {
  useLocale();
  const [internalIdx, setInternalIdx] = useState(0);
  const idx = controlledIdx ?? internalIdx;
  const setIdx = useCallback(
    (next: number) => {
      if (onIdxChange) onIdxChange(next);
      else setInternalIdx(next);
    },
    [onIdxChange],
  );

  const total = envelopes.length;
  const visible = envelopes.slice(0, idx + 1);
  const atEnd = idx >= total - 1;
  const atStart = idx === 0;

  const next = useCallback(() => {
    setIdx(Math.min(idx + 1, total - 1));
  }, [idx, total, setIdx]);
  const prev = useCallback(() => {
    setIdx(Math.max(idx - 1, 0));
  }, [idx, setIdx]);

  // Auto-scroll the latest envelope card into view when the cursor
  // advances, so the fixed-height reply pane behaves like a chat window
  // pinned to the most-recent message.
  const listRef = useRef<HTMLDivElement | null>(null);
  const bottomRef = useRef<HTMLDivElement | null>(null);
  useEffect(() => {
    if (!bottomRef.current) return;
    bottomRef.current.scrollIntoView({ behavior: "smooth", block: "end" });
  }, [idx]);

  if (total === 0) {
    return (
      <div
        style={{
          padding: "32px 24px",
          textAlign: "center",
          color: "var(--muted)",
          fontSize: 13,
        }}
      >
        {emptyHint ?? t("step_empty")}
      </div>
    );
  }

  return (
    <div style={{ display: "flex", flexDirection: "column", height: "100%", minHeight: 0 }}>
      <div
        ref={listRef}
        className="reply-scroll"
        style={{
          flex: "1 1 auto",
          minHeight: 0,
          overflowY: "auto",
          overflowX: "hidden",
          display: "flex",
          flexDirection: "column",
          gap: 10,
          padding: "4px 2px 12px",
        }}
      >
        {visible.map((env, i) => (
          <div
            key={`${env.session_id}-${env.seq}`}
            style={{
              opacity: 0,
              animation: "reply-fade-in 0.35s ease-out forwards",
              animationDelay: i === visible.length - 1 ? "0s" : "0s",
            }}
          >
            <EnvelopeCard env={env} />
          </div>
        ))}
        <div ref={bottomRef} />
      </div>

      <div
        style={{
          flex: "0 0 auto",
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          padding: "10px 12px",
          borderTop: "1px solid var(--divider)",
          background: "var(--surface)",
        }}
      >
        <button
          type="button"
          onClick={prev}
          disabled={atStart}
          style={controlBtnStyle(atStart)}
        >
          <ChevronLeft size={14} strokeWidth={2} />
          {t("nav_prev")}
        </button>
        <span
          style={{
            fontSize: 12,
            color: "var(--muted)",
            fontVariantNumeric: "tabular-nums",
          }}
        >
          {idx + 1} / {total}
        </span>
        <button
          type="button"
          onClick={next}
          disabled={atEnd}
          style={controlBtnStyle(atEnd)}
        >
          {t("nav_next")}
          <ChevronRight size={14} strokeWidth={2} />
        </button>
      </div>

      <style jsx>{`
        @keyframes reply-fade-in {
          from {
            opacity: 0;
            transform: translateY(6px);
          }
          to {
            opacity: 1;
            transform: translateY(0);
          }
        }
      `}</style>
    </div>
  );
}

function controlBtnStyle(disabled: boolean): React.CSSProperties {
  return {
    display: "inline-flex",
    alignItems: "center",
    gap: 4,
    padding: "5px 12px",
    fontSize: 12.5,
    fontWeight: 500,
    background: disabled ? "var(--surface-2)" : "var(--accent)",
    color: disabled ? "var(--muted)" : "var(--accent-fg)",
    border: "1px solid var(--divider)",
    borderRadius: 5,
    cursor: disabled ? "not-allowed" : "pointer",
    opacity: disabled ? 0.6 : 1,
  };
}
