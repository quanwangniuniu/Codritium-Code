"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import ReactMarkdown from "react-markdown";
import { Lightbulb } from "lucide-react";
import { workspaceApi, type TipsMessage } from "@/features/workspace/api";
import { toast } from "@/shared/lib/toast";
import { t } from "@/shared/i18n";
import { useComposer } from "../hooks/useComposer";
import { ChatComposer } from "./ChatComposer";

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
  const [busy, setBusy] = useState(false);
  const scrollRef = useRef<HTMLDivElement>(null);
  const streamingKeyRef = useRef<string | null>(null);

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
        const resp = await workspaceApi.tipsMessages(sessionId);
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
        await workspaceApi.tipsStream(
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

  const composer = useComposer({
    busy,
    onSend: send,
    files: availableFiles,
    disabled: submitted,
    onSubmitted: onTipsTouched,
  });

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
        {composer.queue.length > 0 && (
          <div style={{ color: AMBER_ACCENT, fontSize: 11.5 }}>
            queued: {composer.queue.length}
          </div>
        )}
      </div>

      <ChatComposer
        composer={composer}
        variant="tutor"
        busy={busy}
        placeholder={placeholder}
        disabled={submitted}
      />
    </section>
  );
}
