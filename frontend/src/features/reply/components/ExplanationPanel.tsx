"use client";

import ReactMarkdown from "react-markdown";
import { t } from "@/shared/i18n";
import { useLocale } from "@/shared/i18n/client";

// ExplanationPanel renders the standard-solution prose underneath the
// diff viewer. The replay UI surfaces the engineer's first-person
// reasoning step by step on the right; this panel is the canonical
// after-the-fact explanation that stays visible across the full timeline.

export function ExplanationPanel({
  markdown,
  compact = false,
}: {
  markdown: string;
  compact?: boolean;
}) {
  useLocale();
  if (!markdown) {
    return (
      <div
        style={{
          padding: "16px",
          color: "var(--muted)",
          fontSize: 12,
          fontStyle: "italic",
        }}
      >
        {t("explanation_empty")}
      </div>
    );
  }
  return (
    <div
      style={{
        padding: compact ? "12px 14px" : "16px 18px",
        fontSize: compact ? 11.5 : 13,
        lineHeight: compact ? 1.55 : 1.6,
        color: compact ? "var(--muted)" : "var(--ink)",
      }}
      className="reply-explanation"
    >
      <ReactMarkdown>{markdown}</ReactMarkdown>
      <style jsx>{`
        :global(.reply-explanation h1),
        :global(.reply-explanation h2),
        :global(.reply-explanation h3) {
          font-weight: 600;
          margin-top: 18px;
          margin-bottom: 8px;
          line-height: 1.3;
        }
        :global(.reply-explanation h1) {
          font-size: 17px;
        }
        :global(.reply-explanation h2) {
          font-size: 15px;
        }
        :global(.reply-explanation h3) {
          font-size: 13.5px;
          color: var(--muted);
        }
        :global(.reply-explanation p) {
          margin: 8px 0;
        }
        :global(.reply-explanation code) {
          font-family: var(--font-jetbrains-mono), ui-monospace, monospace;
          font-size: 12px;
          padding: 1px 5px;
          border-radius: 3px;
          background: var(--surface-2, #f6f7f8);
        }
        :global(.reply-explanation pre) {
          background: var(--surface-2, #f6f7f8);
          border: 1px solid var(--divider);
          border-radius: 5px;
          padding: 10px 12px;
          overflow-x: auto;
          font-size: 12px;
        }
        :global(.reply-explanation pre code) {
          background: transparent;
          padding: 0;
        }
        :global(.reply-explanation ul),
        :global(.reply-explanation ol) {
          margin: 8px 0;
          padding-left: 22px;
        }
        :global(.reply-explanation li) {
          margin: 3px 0;
        }
        :global(.reply-explanation strong) {
          font-weight: 600;
        }
      `}</style>
    </div>
  );
}
