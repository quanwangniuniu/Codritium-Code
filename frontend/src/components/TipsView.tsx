"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import ReactMarkdown from "react-markdown";
import { Lightbulb, Send, AtSign } from "lucide-react";
import { Backend, type TipsMessage } from "@/lib/api";
import { toast } from "@/lib/toast";
import { t } from "@/lib/i18n";

// Tints used to mark the tutor surface visually distinct from the agent
// surface. R12 cursor research recommends "diff lives where the cursor
// lives"; we extend it to "tutor lives in a different colour" so the
// candidate can tell from a glance which conversation they are in.
const AMBER_TINT_BG = "rgba(245, 158, 11, 0.06)";
const AMBER_BORDER = "rgba(245, 158, 11, 0.35)";
const AMBER_ACCENT = "#f59e0b";

type LocalRow = {
  // localKey distinguishes optimistic rows (not yet persisted) from
  // stored rows. Stored rows carry the server-assigned seq; in-progress
  // streaming model turns carry a negative client seq to avoid colliding.
  localKey: string;
  role: "user" | "model";
  text: string;
  streaming?: boolean;
};

function toLocal(msg: TipsMessage): LocalRow {
  return {
    localKey: `srv-${msg.seq}`,
    role: msg.role,
    text: msg.text,
  };
}

export function TipsView({
  sessionId,
  submitted,
  fileContents,
  availableFiles,
  onTipsTouched,
}: {
  sessionId: string | null;
  submitted: boolean;
  fileContents: Record<string, string>;
  availableFiles: string[];
  onTipsTouched?: () => void;
}) {
  const [rows, setRows] = useState<LocalRow[]>([]);
  const [text, setText] = useState("");
  const [busy, setBusy] = useState(false);
  const [queue, setQueue] = useState<string[]>([]);
  const [mention, setMention] = useState<{ query: string; start: number } | null>(null);
  const scrollRef = useRef<HTMLDivElement>(null);
  const prevBusyRef = useRef(busy);
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const streamingKeyRef = useRef<string | null>(null);

  const mentionMatches = (() => {
    if (!mention) return [];
    const q = mention.query.toLowerCase();
    return availableFiles.filter((f) => f.toLowerCase().includes(q)).slice(0, 6);
  })();

  // Hydrate persisted tutor conversation when the session id resolves
  // or changes (resume path covers refresh + cross-device practice).
  useEffect(() => {
    if (!sessionId) {
      setRows([]);
      return;
    }
    let cancelled = false;
    (async () => {
      try {
        const resp = await Backend.tipsMessages(sessionId);
        if (cancelled) return;
        setRows(resp.messages.map(toLocal));
      } catch {
        // Resume failures are non-fatal — empty state is acceptable for
        // a fresh session, and the next send will surface real errors.
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [sessionId]);

  useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [rows]);

  const send = useCallback(
    async (message: string) => {
      if (!sessionId) return;
      const userKey = `local-user-${Date.now()}`;
      const streamKey = `local-model-${Date.now()}-stream`;
      streamingKeyRef.current = streamKey;
      setRows((prev) => [
        ...prev,
        { localKey: userKey, role: "user", text: message },
        { localKey: streamKey, role: "model", text: "", streaming: true },
      ]);
      setBusy(true);
      try {
        await Backend.tipsStream(
          sessionId,
          message,
          fileContents,
          {
            onDelta: (delta) => {
              setRows((prev) =>
                prev.map((r) =>
                  r.localKey === streamKey
                    ? { ...r, text: r.text + delta }
                    : r,
                ),
              );
            },
            onDone: () => {
              setRows((prev) =>
                prev.map((r) =>
                  r.localKey === streamKey
                    ? { ...r, streaming: false }
                    : r,
                ),
              );
            },
            onError: (msg) => {
              setRows((prev) =>
                prev.filter((r) => r.localKey !== streamKey),
              );
              toast.error(`${t("tutor_send_failed")} (${msg})`);
            },
          },
        );
      } catch (err) {
        const msg = err instanceof Error ? err.message : String(err);
        setRows((prev) => prev.filter((r) => r.localKey !== streamKey));
        toast.error(`${t("tutor_send_failed")} (${msg})`);
      } finally {
        setBusy(false);
        streamingKeyRef.current = null;
      }
    },
    [sessionId, fileContents],
  );

  // Drain queued messages when the model finishes the current turn.
  useEffect(() => {
    const wasBusy = prevBusyRef.current;
    prevBusyRef.current = busy;
    if (wasBusy && !busy && queue.length > 0) {
      const [next, ...rest] = queue;
      setQueue(rest);
      void send(next);
    }
  }, [busy, queue, send]);

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    const v = text.trim();
    if (!v || submitted) return;
    setText("");
    setMention(null);
    onTipsTouched?.();
    if (busy) {
      setQueue((q) => [...q, v]);
      return;
    }
    void send(v);
  };

  const handleTextChange = (e: React.ChangeEvent<HTMLTextAreaElement>) => {
    const val = e.target.value;
    setText(val);
    const caret = e.target.selectionStart ?? val.length;
    const before = val.slice(0, caret);
    const at = before.lastIndexOf("@");
    if (at < 0) {
      setMention(null);
      return;
    }
    const between = before.slice(at + 1);
    if (between.includes(" ") || between.includes("\n")) {
      setMention(null);
      return;
    }
    setMention({ query: between, start: at });
  };

  const pickMention = (name: string) => {
    if (!mention || !textareaRef.current) return;
    const caret = textareaRef.current.selectionStart ?? text.length;
    const head = text.slice(0, mention.start);
    const tail = text.slice(caret);
    const replaced = `${head}@${name} ${tail}`;
    setText(replaced);
    setMention(null);
    requestAnimationFrame(() => {
      if (textareaRef.current) {
        const pos = head.length + name.length + 2;
        textareaRef.current.focus();
        textareaRef.current.setSelectionRange(pos, pos);
      }
    });
  };

  const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (mention && mentionMatches.length > 0) {
      if (e.key === "Tab" || e.key === "Enter") {
        e.preventDefault();
        pickMention(mentionMatches[0]);
        return;
      }
      if (e.key === "Escape") {
        setMention(null);
        return;
      }
    }
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      handleSubmit(e as unknown as React.FormEvent);
    }
  };

  const placeholder = submitted ? t("tutor_submitted") : t("ask_tutor");

  return (
    <section
      style={{
        flex: 1,
        display: "flex",
        flexDirection: "column",
        minWidth: 0,
        background: AMBER_TINT_BG,
        borderTop: `1px solid ${AMBER_BORDER}`,
      }}
    >
      <header
        style={{
          display: "flex",
          alignItems: "center",
          gap: 8,
          padding: "10px 12px",
          borderBottom: `1px solid ${AMBER_BORDER}`,
          color: "var(--text-strong)",
          fontSize: 12.5,
        }}
      >
        <Lightbulb size={15} color={AMBER_ACCENT} strokeWidth={2} />
        <span style={{ fontWeight: 600 }}>{t("tutor_role")}</span>
        <span style={{ color: "var(--text-muted)", fontSize: 11.5 }}>
          · {t("tutor_empty_state").split(".")[0]}.
        </span>
      </header>

      <div
        ref={scrollRef}
        className="scroll-y"
        style={{ flex: 1, padding: "12px", display: "flex", flexDirection: "column", gap: 10 }}
      >
        {rows.length === 0 && (
          <div
            style={{
              color: "var(--text-muted)",
              fontSize: 12.5,
              lineHeight: 1.6,
              padding: "16px 12px",
              border: `1px dashed ${AMBER_BORDER}`,
              borderRadius: 6,
              background: "rgba(245, 158, 11, 0.04)",
            }}
          >
            {t("tutor_empty_state")}
          </div>
        )}
        {rows.map((r) => (
          <div
            key={r.localKey}
            style={{
              display: "flex",
              flexDirection: "column",
              gap: 4,
              alignItems: r.role === "user" ? "flex-end" : "flex-start",
            }}
          >
            <div
              style={{
                fontSize: 11,
                color: "var(--text-muted)",
                paddingLeft: 4,
                paddingRight: 4,
              }}
            >
              {r.role === "user" ? "you" : t("tutor_role")}
            </div>
            <div
              style={{
                background:
                  r.role === "user" ? "var(--bg-tab)" : "rgba(245, 158, 11, 0.10)",
                border: `1px solid ${
                  r.role === "user" ? "var(--border-soft)" : AMBER_BORDER
                }`,
                color: "var(--text)",
                padding: "8px 10px",
                borderRadius: 6,
                maxWidth: "90%",
                fontSize: 12.5,
                lineHeight: 1.55,
                whiteSpace: "pre-wrap",
                wordBreak: "break-word",
              }}
            >
              {r.role === "model" ? (
                <ReactMarkdown>
                  {r.text || (r.streaming ? "…" : "")}
                </ReactMarkdown>
              ) : (
                r.text
              )}
            </div>
          </div>
        ))}
        {busy && !streamingKeyRef.current && (
          <div style={{ color: "var(--text-muted)", fontSize: 12 }}>
            {t("tutor_thinking")}
          </div>
        )}
        {queue.length > 0 && (
          <div style={{ color: AMBER_ACCENT, fontSize: 11.5 }}>
            queued: {queue.length}
          </div>
        )}
      </div>

      <form
        onSubmit={handleSubmit}
        style={{
          padding: 10,
          borderTop: `1px solid ${AMBER_BORDER}`,
          display: "flex",
          flexDirection: "column",
          gap: 6,
          position: "relative",
        }}
      >
        {mention && mentionMatches.length > 0 && (
          <div
            style={{
              position: "absolute",
              bottom: "100%",
              left: 10,
              right: 10,
              background: "var(--bg-side)",
              border: `1px solid ${AMBER_BORDER}`,
              borderRadius: 6,
              padding: 4,
              display: "flex",
              flexDirection: "column",
              gap: 2,
              maxHeight: 180,
              overflowY: "auto",
              zIndex: 10,
            }}
          >
            {mentionMatches.map((m, idx) => (
              <button
                key={m}
                type="button"
                onClick={() => pickMention(m)}
                style={{
                  display: "flex",
                  alignItems: "center",
                  gap: 6,
                  padding: "4px 6px",
                  borderRadius: 4,
                  background: idx === 0 ? "var(--bg-tab)" : "transparent",
                  color: "var(--text)",
                  fontSize: 12,
                  textAlign: "left",
                  fontFamily: "var(--font-jetbrains-mono), ui-monospace, monospace",
                }}
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
          onChange={handleTextChange}
          onKeyDown={handleKeyDown}
          placeholder={placeholder}
          disabled={submitted}
          rows={3}
          style={{
            width: "100%",
            background: "var(--bg-app)",
            color: "var(--text)",
            border: `1px solid ${submitted ? "var(--border-soft)" : AMBER_BORDER}`,
            borderRadius: 6,
            padding: "8px 10px",
            fontSize: 12.5,
            fontFamily: "inherit",
            resize: "none",
            outline: "none",
            opacity: submitted ? 0.6 : 1,
          }}
        />
        <div style={{ display: "flex", justifyContent: "flex-end" }}>
          <button
            type="submit"
            disabled={submitted || !text.trim()}
            style={{
              display: "flex",
              alignItems: "center",
              gap: 5,
              padding: "5px 10px",
              borderRadius: 5,
              border: "none",
              background:
                submitted || !text.trim() ? "var(--bg-tab)" : AMBER_ACCENT,
              color: submitted || !text.trim() ? "var(--text-muted)" : "#1a1100",
              fontSize: 12,
              fontWeight: 600,
              cursor: submitted || !text.trim() ? "not-allowed" : "pointer",
            }}
          >
            <Send size={12} strokeWidth={2.2} />
            {busy ? "Queue" : "Send"}
          </button>
        </div>
      </form>
    </section>
  );
}
