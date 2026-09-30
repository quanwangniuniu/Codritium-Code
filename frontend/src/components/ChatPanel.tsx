"use client";

import { useState, useRef, useEffect } from "react";
import ReactMarkdown from "react-markdown";
import { Sparkles, Send, AtSign, Check, X } from "lucide-react";
import { t } from "@/shared/i18n";
import { useLocale } from "@/shared/i18n/client";
import { PatchPreview, type PendingPatch, type ResolveCallback } from "@/components/PatchPreview";

export type TextMessage = {
  // kind is optional for backwards compatibility with sessions persisted
  // before patch messages were introduced. Treat missing kind as "text".
  kind?: "text";
  id: string;
  role: "user" | "assistant";
  content: string;
  streaming?: boolean;
};

export type PatchMessage = {
  kind: "patch";
  id: string;
  pending: PendingPatch;
  resolved?: { kind: "approve" | "modify" | "reject" };
};

export type ChatMessage = TextMessage | PatchMessage;

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
  const [text, setText] = useState("");
  const [queue, setQueue] = useState<string[]>([]);
  const [mention, setMention] = useState<{ query: string; start: number } | null>(null);
  const prevBusyRef = useRef(busy);
  const scrollRef = useRef<HTMLDivElement>(null);
  const textareaRef = useRef<HTMLTextAreaElement>(null);

  const mentionMatches = (() => {
    if (!mention || !availableFiles) return [];
    const q = mention.query.toLowerCase();
    return availableFiles.filter((f) => f.toLowerCase().includes(q)).slice(0, 6);
  })();

  useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [messages]);

  // Auto-dispatch queued message when the agent transitions from
  // busy → idle, so Enter-during-streaming behaves like a queue.
  useEffect(() => {
    const wasBusy = prevBusyRef.current;
    prevBusyRef.current = busy;
    if (wasBusy && !busy && queue.length > 0) {
      const next = queue[0];
      setQueue((q) => q.slice(1));
      onSend(next);
    }
  }, [busy, queue, onSend]);

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    const t = text.trim();
    if (!t) return;
    setText("");
    setMention(null);
    if (busy) {
      setQueue((q) => [...q, t]);
      return;
    }
    onSend(t);
  };

  const handleTextChange = (e: React.ChangeEvent<HTMLTextAreaElement>) => {
    const next = e.target.value;
    setText(next);
    const cursor = e.target.selectionStart ?? next.length;
    const before = next.slice(0, cursor);
    const m = before.match(/@(\S*)$/);
    if (m) {
      setMention({ query: m[1], start: cursor - m[0].length });
    } else {
      setMention(null);
    }
  };

  const selectMention = (filename: string) => {
    if (!mention) return;
    const before = text.slice(0, mention.start);
    const after = text.slice((textareaRef.current?.selectionStart ?? text.length));
    const next = `${before}@${filename} ${after}`;
    setText(next);
    setMention(null);
    requestAnimationFrame(() => {
      const ta = textareaRef.current;
      if (ta) {
        const pos = before.length + filename.length + 2; // @ + name + space
        ta.focus();
        ta.setSelectionRange(pos, pos);
      }
    });
  };

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

      <form
        onSubmit={handleSubmit}
        style={{
          borderTop: "1px solid var(--border)",
          padding: "10px 12px",
          background: "var(--bg-side)",
          flexShrink: 0,
        }}
      >
        <div
          style={{
            display: "flex",
            alignItems: "flex-start",
            gap: 6,
            background: "var(--bg-editor)",
            border: "1px solid var(--border)",
            borderRadius: 6,
            padding: "7px 10px",
          }}
        >
          <AtSign size={13} strokeWidth={1.5} style={{ color: "var(--text-muted)", marginTop: 2 }} />
          <textarea
            ref={textareaRef}
            value={text}
            onChange={handleTextChange}
            placeholder={busy ? t("ai_thinking") : t("ask_ai")}
            rows={2}
            style={{
              flex: 1,
              background: "transparent",
              border: "none",
              outline: "none",
              color: "var(--text)",
              fontFamily: "inherit",
              fontSize: 12.5,
              resize: "none",
            }}
            onKeyDown={(e) => {
              if (mention && mentionMatches.length > 0) {
                if (e.key === "Tab" || (e.key === "Enter" && !e.shiftKey)) {
                  e.preventDefault();
                  selectMention(mentionMatches[0]);
                  return;
                }
                if (e.key === "Escape") {
                  e.preventDefault();
                  setMention(null);
                  return;
                }
              }
              if (e.key === "Enter" && !e.shiftKey) {
                e.preventDefault();
                handleSubmit(e as unknown as React.FormEvent);
              }
            }}
          />
        </div>
        <div
          style={{
            display: "flex",
            alignItems: "center",
            justifyContent: "space-between",
            marginTop: 6,
          }}
        >
          <span style={{ fontSize: 10, color: "var(--text-muted)" }}>
            {busy ? t("chat_enter_to_queue_hint") : t("chat_enter_to_send_hint")}
          </span>
          <button
            type="submit"
            disabled={!text.trim()}
            style={{
              padding: "4px 12px",
              fontSize: 11.5,
              background: text.trim() ? (busy ? "var(--bg-tab)" : "var(--accent)") : "var(--bg-tab)",
              color: text.trim() ? (busy ? "var(--text)" : "#ffffff") : "var(--text-muted)",
              borderRadius: 3,
              fontWeight: 600,
              display: "inline-flex",
              alignItems: "center",
              gap: 6,
            }}
          >
            <Send size={12} strokeWidth={1.7} />
            <span>{busy ? t("chat_queue_btn") : t("chat_send_btn")}</span>
          </button>
        </div>
        {mention && mentionMatches.length > 0 && (
          <div
            style={{
              marginTop: 6,
              border: "1px solid var(--border)",
              background: "var(--bg-editor)",
              borderRadius: 4,
              overflow: "hidden",
            }}
          >
            {mentionMatches.map((f, i) => (
              <button
                key={f}
                type="button"
                onMouseDown={(e) => {
                  e.preventDefault();
                  selectMention(f);
                }}
                style={{
                  display: "block",
                  width: "100%",
                  textAlign: "left",
                  padding: "5px 10px",
                  fontSize: 11.5,
                  background: i === 0 ? "rgba(0,122,204,0.10)" : "transparent",
                  color: "var(--text)",
                  border: "none",
                  cursor: "pointer",
                  fontFamily: "var(--font-mono)",
                }}
              >
                {f}
                {i === 0 && (
                  <span
                    style={{
                      marginLeft: 8,
                      fontSize: 10,
                      color: "var(--text-muted)",
                      fontFamily: "inherit",
                    }}
                  >
                    {t("chat_tab_enter")}
                  </span>
                )}
              </button>
            ))}
          </div>
        )}
        {queue.length > 0 && (
          <div
            style={{
              marginTop: 6,
              display: "flex",
              flexWrap: "wrap",
              gap: 4,
            }}
          >
            {queue.map((q, i) => (
              <span
                key={i}
                title={q}
                style={{
                  fontSize: 10.5,
                  color: "var(--text-muted)",
                  background: "var(--bg-editor)",
                  border: "1px dashed var(--border)",
                  padding: "2px 6px",
                  borderRadius: 3,
                  maxWidth: 220,
                  overflow: "hidden",
                  textOverflow: "ellipsis",
                  whiteSpace: "nowrap",
                }}
              >
                {t("chat_queued_prefix")} {q.slice(0, 32)}{q.length > 32 ? "…" : ""}
              </span>
            ))}
          </div>
        )}
      </form>
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
