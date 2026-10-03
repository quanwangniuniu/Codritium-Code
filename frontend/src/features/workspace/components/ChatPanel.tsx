"use client";

import { useEffect, useRef, useState } from "react";
import ReactMarkdown from "react-markdown";
import { Check, ChevronDown, ChevronRight, Sparkles, X } from "lucide-react";
import { t } from "@/shared/i18n";
import { useLocale } from "@/shared/i18n/client";
import { useComposer } from "../hooks/useComposer";
import { ChatComposer } from "./ChatComposer";
import { PatchPreview } from "./PatchPreview";
import type { ChatMessage, PatchMessage, ResolveCallback } from "../types";

function isPatch(m: ChatMessage): m is PatchMessage {
  return (m as PatchMessage).kind === "patch";
}

export function ChatPanel({
  messages,
  onSend,
  busy,
  onApply,
  onPatchResolved,
  getPendingContent,
  availableFiles,
}: {
  messages: ChatMessage[];
  onSend: (text: string) => void;
  busy: boolean;
  onApply?: (codeBlock: string) => void;
  onPatchResolved?: ResolveCallback;
  getPendingContent?: (toolUseId: string) => string | undefined;
  availableFiles?: string[];
}) {
  useLocale();
  const scrollRef = useRef<HTMLDivElement>(null);
  const composer = useComposer({ busy, onSend, files: availableFiles });

  useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [messages]);

  return (
    <aside
      style={{
        width: "100%",
        height: "100%",
        background: "var(--bg-chat)",
        display: "flex",
        flexDirection: "column",
        flexShrink: 0,
      }}
    >
      <div
        style={{
          padding: "9px 14px 8px",
          fontSize: 11,
          color: "var(--text-dim)",
          textTransform: "uppercase",
          letterSpacing: "0.06em",
          background: "var(--bg-side)",
          borderBottom: "1px solid var(--border)",
          display: "flex",
          alignItems: "center",
          gap: 8,
          flexShrink: 0,
          fontWeight: 600,
        }}
      >
        <Sparkles size={13} strokeWidth={1.7} className="text-ide-accent" />
        <span>{t("chat_ai_assistant")}</span>
        <span className="flex-1" />
        <span
          className="mono"
          style={{
            fontSize: 10,
            color: "var(--text-muted)",
            textTransform: "none",
          }}
        >
          ⌘ L
        </span>
      </div>

      <div ref={scrollRef} className="scroll-y flex-1 px-3.5 py-3">
        {messages.length === 0 && (
          <div
            style={{
              color: "var(--text-dim)",
              fontSize: 12.5,
              lineHeight: 1.65,
            }}
          >
            {t("chat_subtitle")}
          </div>
        )}

        {messages.map((m) => {
          if (isPatch(m)) {
            return (
              <PatchTimelineEntry
                key={m.id}
                message={m}
                onResolved={onPatchResolved}
                getPendingContent={getPendingContent}
              />
            );
          }
          return (
            <div
              key={m.id}
              style={{
                marginBottom: 14,
                padding: "10px 12px",
                borderRadius: 6,
                background:
                  m.role === "user" ? "rgba(0,122,204,0.10)" : "var(--bg-side)",
                border:
                  m.role === "user"
                    ? "1px solid rgba(0,122,204,0.30)"
                    : "1px solid var(--border)",
                fontSize: 12.5,
                lineHeight: 1.65,
                color: "var(--text)",
              }}
            >
              <div
                style={{
                  fontSize: 10,
                  color: "var(--text-muted)",
                  marginBottom: 6,
                  textTransform: "uppercase",
                  letterSpacing: "0.05em",
                  fontWeight: 600,
                }}
              >
                {m.role === "user" ? t("chat_role_you") : t("ai_role")}
                {m.streaming && <span className="ml-1.5 text-ide-warn">…</span>}
              </div>
              <ReactMarkdown
                components={{
                  code: ({ children, className }) => {
                    const text = String(children);
                    const isBlock =
                      /\n/.test(text) || className?.startsWith("language-");
                    if (!isBlock) {
                      return (
                        <code
                          className="mono"
                          style={{
                            background: "var(--bg-editor)",
                            padding: "1px 4px",
                            borderRadius: 3,
                            fontSize: 11,
                            color: "var(--syntax-orange)",
                          }}
                        >
                          {children}
                        </code>
                      );
                    }
                    return (
                      <div className="relative my-2">
                        <pre
                          className="mono"
                          style={{
                            background: "var(--bg-editor)",
                            padding: "10px 12px",
                            borderRadius: 4,
                            fontSize: 11.5,
                            overflow: "auto",
                            margin: 0,
                            border: "1px solid var(--border)",
                            color: "var(--text)",
                          }}
                        >
                          <code>{text}</code>
                        </pre>
                        {onApply && (
                          <button
                            onClick={() => onApply(text)}
                            style={{
                              position: "absolute",
                              top: 6,
                              right: 6,
                              padding: "3px 8px",
                              fontSize: 10.5,
                              background: "var(--accent)",
                              color: "#ffffff",
                              borderRadius: 3,
                              fontWeight: 600,
                            }}
                          >
                            {t("chat_apply_btn")}
                          </button>
                        )}
                      </div>
                    );
                  },
                  p: ({ children }) => <p className="mb-1.5">{children}</p>,
                  ul: ({ children }) => (
                    <ul className="pl-[18px]">{children}</ul>
                  ),
                }}
              >
                {m.content}
              </ReactMarkdown>
            </div>
          );
        })}
      </div>

      <ChatComposer
        composer={composer}
        variant="agent"
        busy={busy}
        placeholder={busy ? t("ai_thinking") : t("ask_ai")}
      />
    </aside>
  );
}

function PatchTimelineEntry({
  message,
  onResolved,
  getPendingContent,
}: {
  message: PatchMessage;
  onResolved?: ResolveCallback;
  getPendingContent?: (toolUseId: string) => string | undefined;
}) {
  const [expanded, setExpanded] = useState(false);

  if (message.resolved) {
    const isReject = message.resolved.kind === "reject";
    const isAuto = message.pending.auto === true;
    const Icon = isReject ? X : Check;
    const label = isAuto
      ? t("patch_label_auto")
      : message.resolved.kind === "approve"
        ? t("patch_label_approved")
        : message.resolved.kind === "modify"
          ? t("patch_label_modified")
          : t("patch_label_rejected");
    return (
      <div className="mb-3.5 overflow-hidden rounded border border-ide-border bg-ide-side">
        <button
          type="button"
          onClick={() => setExpanded((current) => !current)}
          className="flex w-full items-center gap-2 px-3 py-2 text-left text-[11.5px] text-ide-text-dim"
          aria-expanded={expanded}
        >
          <Icon
            size={13}
            strokeWidth={2}
            style={{
              color: isReject
                ? "var(--bad)"
                : isAuto
                  ? "var(--text-muted)"
                  : "var(--good)",
            }}
          />

          <span className="font-semibold text-ide-text">
            {message.pending.tool}
          </span>

          {message.pending.path && (
            <span className="mono min-w-0 flex-1 truncate text-ide-text-muted">
              {message.pending.path}
            </span>
          )}

          {!message.pending.path && <span className="flex-1" />}

          <span
            className={
              isReject
                ? "text-ide-bad"
                : isAuto
                  ? "text-ide-text-muted"
                  : "text-ide-good"
            }
          >
            {label}
          </span>

          {expanded ? (
            <ChevronDown size={13} strokeWidth={1.7} />
          ) : (
            <ChevronRight size={13} strokeWidth={1.7} />
          )}
        </button>

        {expanded && (
          <div className="border-t border-ide-border bg-ide-editor px-3 py-2.5 text-xs">
            {message.pending.inputSummary && (
              <div className="text-ide-text-dim">
                {message.pending.inputSummary}
              </div>
            )}

            {message.toolResult && (
              <div
                className={
                  message.pending.inputSummary
                    ? "mt-2 border-t border-ide-border pt-2"
                    : ""
                }
              >
                <div
                  className={
                    message.toolResult.isError
                      ? "mb-1.5 font-semibold text-ide-bad"
                      : "mb-1.5 font-semibold text-ide-good"
                  }
                >
                  {message.toolResult.isError
                    ? "Tool failed"
                    : "Tool completed"}
                  {message.toolResult.durationMs !== undefined &&
                    ` · ${message.toolResult.durationMs} ms`}
                </div>

                <pre className="mono max-h-48 overflow-auto whitespace-pre-wrap break-words text-[11px] leading-5 text-ide-text">
                  {message.toolResult.summary || "No output"}
                </pre>
              </div>
            )}
          </div>
        )}
      </div>
    );
  }

  return (
    <PatchPreview
      pending={message.pending}
      getPendingContent={getPendingContent}
      onResolved={(toolUseId, kind, result) => {
        onResolved?.(toolUseId, kind, result);
      }}
    />
  );
}
