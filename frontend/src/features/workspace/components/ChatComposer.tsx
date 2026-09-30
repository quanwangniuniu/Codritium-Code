"use client";

import { AtSign, Send } from "lucide-react";
import type { Composer } from "@/features/workspace/hooks/useComposer";
import { t } from "@/shared/i18n";

// Tutor surface tints (kept in sync with TipsView).
const AMBER_BORDER = "rgba(245, 158, 11, 0.35)";
const AMBER_ACCENT = "#f59e0b";

interface ChatComposerProps {
  composer: Composer;
  // "agent": compact input under the AI assistant chat, mention list and
  // queue chips inline. "tutor": amber-tinted input with a floating
  // mention popover (the tutor shows its queue count in the timeline).
  variant: "agent" | "tutor";
  busy: boolean;
  placeholder: string;
  disabled?: boolean;
}

export function ChatComposer(props: ChatComposerProps) {
  return props.variant === "agent" ? <AgentComposer {...props} /> : <TutorComposer {...props} />;
}

function AgentComposer({ composer, busy, placeholder }: ChatComposerProps) {
  const { text, queue, mentionOpen, mentionMatches, textareaRef } = composer;
  const hasText = text.trim().length > 0;
  return (
    <form
      onSubmit={composer.handleSubmit}
      className="shrink-0 border-t border-ide-border bg-ide-side px-3 py-2.5"
    >
      <div className="flex items-start gap-1.5 rounded-md border border-ide-border bg-ide-editor px-2.5 py-[7px]">
        <AtSign size={13} strokeWidth={1.5} className="mt-0.5 text-ide-text-muted" />
        <textarea
          ref={textareaRef}
          value={text}
          onChange={composer.handleChange}
          onKeyDown={composer.handleKeyDown}
          placeholder={placeholder}
          rows={2}
          className="flex-1 resize-none border-none bg-transparent font-[family-name:inherit] text-[12.5px] text-ide-text outline-none"
        />
      </div>
      <div className="mt-1.5 flex items-center justify-between">
        <span className="text-[10px] text-ide-text-muted">
          {busy ? t("chat_enter_to_queue_hint") : t("chat_enter_to_send_hint")}
        </span>
        <button
          type="submit"
          disabled={!hasText}
          className="inline-flex items-center gap-1.5 rounded-[3px] px-3 py-1 text-[11.5px] font-semibold"
          style={{
            background: hasText ? (busy ? "var(--bg-tab)" : "var(--accent)") : "var(--bg-tab)",
            color: hasText ? (busy ? "var(--text)" : "#ffffff") : "var(--text-muted)",
          }}
        >
          <Send size={12} strokeWidth={1.7} />
          <span>{busy ? t("chat_queue_btn") : t("chat_send_btn")}</span>
        </button>
      </div>
      {mentionOpen && (
        <div className="mt-1.5 overflow-hidden rounded border border-ide-border bg-ide-editor">
          {mentionMatches.map((f, i) => (
            <button
              key={f}
              type="button"
              onMouseDown={(e) => {
                e.preventDefault();
                composer.selectMention(f);
              }}
              className={`block w-full cursor-pointer border-none px-2.5 py-[5px] text-left font-[family-name:var(--font-mono)] text-[11.5px] text-ide-text ${
                i === 0 ? "bg-[rgba(0,122,204,0.10)]" : "bg-transparent"
              }`}
            >
              {f}
              {i === 0 && (
                <span className="ml-2 font-[family-name:inherit] text-[10px] text-ide-text-muted">{t("chat_tab_enter")}</span>
              )}
            </button>
          ))}
        </div>
      )}
      {queue.length > 0 && (
        <div className="mt-1.5 flex flex-wrap gap-1">
          {queue.map((q, i) => (
            <span
              key={i}
              title={q}
              className="max-w-[220px] overflow-hidden text-ellipsis whitespace-nowrap rounded-[3px] border border-dashed border-ide-border bg-ide-editor px-1.5 py-0.5 text-[10.5px] text-ide-text-muted"
            >
              {t("chat_queued_prefix")} {q.slice(0, 32)}
              {q.length > 32 ? "…" : ""}
            </span>
          ))}
        </div>
      )}
    </form>
  );
}

function TutorComposer({ composer, busy, placeholder, disabled }: ChatComposerProps) {
  const { text, mentionOpen, mentionMatches, textareaRef } = composer;
  const inactive = disabled || !text.trim();
  return (
    <form
      onSubmit={composer.handleSubmit}
      className="relative flex flex-col gap-1.5 p-2.5"
      style={{ borderTop: `1px solid ${AMBER_BORDER}` }}
    >
      {mentionOpen && (
        <div
          className="absolute inset-x-2.5 bottom-full z-10 flex max-h-[180px] flex-col gap-0.5 overflow-y-auto rounded-md bg-ide-side p-1"
          style={{ border: `1px solid ${AMBER_BORDER}` }}
        >
          {mentionMatches.map((m, idx) => (
            <button
              key={m}
              type="button"
              onClick={() => composer.selectMention(m)}
              className={`flex items-center gap-1.5 rounded px-1.5 py-1 text-left font-[family-name:var(--font-jetbrains-mono),ui-monospace,monospace] text-xs text-ide-text ${
                idx === 0 ? "bg-ide-tab" : "bg-transparent"
              }`}
            >
              <AtSign size={11} color={AMBER_ACCENT} />
              {m}
            </button>
          ))}
        </div>
      )}
      <textarea
        ref={textareaRef}
        value={text}
        onChange={composer.handleChange}
        onKeyDown={composer.handleKeyDown}
        placeholder={placeholder}
        disabled={disabled}
        rows={3}
        className="w-full resize-none rounded-md bg-ide-app px-2.5 py-2 font-[family-name:inherit] text-[12.5px] text-ide-text outline-none"
        style={{
          border: `1px solid ${disabled ? "var(--border-soft)" : AMBER_BORDER}`,
          opacity: disabled ? 0.6 : 1,
        }}
      />
      <div className="flex justify-end">
        <button
          type="submit"
          disabled={inactive}
          className="flex items-center gap-[5px] rounded-[5px] border-none px-2.5 py-[5px] text-xs font-semibold"
          style={{
            background: inactive ? "var(--bg-tab)" : AMBER_ACCENT,
            color: inactive ? "var(--text-muted)" : "#1a1100",
            cursor: inactive ? "not-allowed" : "pointer",
          }}
        >
          <Send size={12} strokeWidth={2.2} />
          {busy ? t("chat_queue_btn") : t("chat_send_btn")}
        </button>
      </div>
    </form>
  );
}
