"use client";

import { useEffect, useRef } from "react";
import ReactMarkdown from "react-markdown";
import { Sparkles, Check, X } from "lucide-react";
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
  availableFiles,
}: {
  messages: ChatMessage[];
  onSend: (text: string) => void;
  busy: boolean;
  onApply?: (codeBlock: string) => void;
  onPatchResolved?: ResolveCallback;
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
        <Sparkles size={13} strokeWidth={1.7} style={{ color: "var(--accent)" }} />
        <span>{t("chat_ai_assistant")}</span>
        <span style={{ flex: 1 }} />
        <span
          className="mono"
          style={{ fontSize: 10, color: "var(--text-muted)", textTransform: "none" }}
        >
          ⌘ L
        </span>
      </div>

      <div ref={scrollRef} className="scroll-y" style={{ flex: 1, padding: "12px 14px" }}>
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
              {m.streaming && (
                <span style={{ marginLeft: 6, color: "var(--warn)" }}>…</span>
              )}
            </div>
            <ReactMarkdown
              components={{
                code: ({ children, className }) => {
                  const text = String(children);
                  const isBlock = /\n/.test(text) || className?.startsWith("language-");
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
                    <div style={{ position: "relative", margin: "8px 0" }}>
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
                p: ({ children }) => <p style={{ marginBottom: 6 }}>{children}</p>,
                ul: ({ children }) => (
                  <ul style={{ paddingLeft: 18 }}>{children}</ul>
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
}: {
  message: PatchMessage;
  onResolved?: ResolveCallback;
}) {
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
      <div
        style={{
          marginBottom: 14,
          padding: "8px 12px",
          borderRadius: 6,
          background: "var(--bg-side)",
          border: "1px solid var(--border)",
          fontSize: 11.5,
          color: "var(--text-dim)",
          display: "flex",
          alignItems: "center",
          gap: 8,
        }}
      >
        <Icon
          size={13}
          strokeWidth={2}
          style={{ color: isReject ? "var(--bad)" : isAuto ? "var(--text-muted)" : "var(--good)" }}
        />
        <span style={{ fontWeight: 600, color: "var(--text)" }}>{label}</span>
        <span style={{ color: "var(--text-muted)" }}>
          {message.pending.tool}
          {message.pending.path ? ` · ${message.pending.path}` : ""}
          {message.pending.tool !== "FileRead" && message.pending.tool !== "FileEdit" && message.pending.inputSummary
            ? ` · ${message.pending.inputSummary}`
            : ""}
        </span>
      </div>
    );
  }

  return (
    <div style={{ marginBottom: 14 }}>
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
        {t("ai_role")} · {t("chat_proposed_word")} {message.pending.tool}
      </div>
      <PatchPreview
        pending={message.pending}
        onResolved={(toolUseId, kind, result) => {
          onResolved?.(toolUseId, kind, result);
        }}
      />
    </div>
  );
}
