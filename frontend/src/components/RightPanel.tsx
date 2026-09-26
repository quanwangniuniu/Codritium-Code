"use client";

import { useEffect, useState } from "react";
import { Lightbulb, MessageSquare } from "lucide-react";
import { ChatPanel, type ChatMessage } from "@/components/ChatPanel";
import { TipsView } from "@/components/TipsView";
import { type ResolveCallback } from "@/components/PatchPreview";
import { t } from "@/lib/i18n";

// Tab label colour used by the Tutor surface; matches TipsView amber so
// switching tabs feels chromatic, not just textual.
const AMBER_ACCENT = "#f59e0b";
const AMBER_TINT_TAB_ACTIVE = "rgba(245, 158, 11, 0.10)";

// localStorage flag retired the 7-minute idle nudge in favour of an
// entry-highlight on the Tutor tab. The value is set once the candidate
// clicks the tutor tab, and read back on mount so a returning session
// doesn't re-flash.
function tutorTouchedKey(sessionId: string | null): string | null {
  if (!sessionId) return null;
  return `tutor-tab-touched:${sessionId}`;
}

type Tab = "agent" | "tips";

export function RightPanel({
  sessionId,
  submitted,
  fileContents,
  fileNames,
  // ChatPanel passthrough
  messages,
  onSend,
  busy,
  onApply,
  onPatchResolved,
}: {
  sessionId: string | null;
  submitted: boolean;
  fileContents: Record<string, string>;
  fileNames: string[];
  messages: ChatMessage[];
  onSend: (text: string) => void;
  busy: boolean;
  onApply?: (codeBlock: string) => void;
  onPatchResolved?: ResolveCallback;
}) {
  const [active, setActive] = useState<Tab>("agent");
  // Tutor tab starts highlighted when the session opens. The 5-minute idle
  // detector is retired (interv2 #9); a single dot fades the first time
  // the candidate visits the tab, and the localStorage flag preserves the
  // state across reloads of the same session.
  const [tipsHighlighted, setTipsHighlighted] = useState(true);

  useEffect(() => {
    const key = tutorTouchedKey(sessionId);
    if (!key || typeof window === "undefined") return;
    if (window.localStorage.getItem(key) === "1") {
      setTipsHighlighted(false);
    } else {
      setTipsHighlighted(true);
    }
  }, [sessionId]);

  const markTipsTouched = () => {
    setTipsHighlighted(false);
    const key = tutorTouchedKey(sessionId);
    if (key && typeof window !== "undefined") {
      window.localStorage.setItem(key, "1");
    }
  };

  const switchTo = (next: Tab) => {
    setActive(next);
    if (next === "tips") markTipsTouched();
  };

  return (
    <div
      style={{
        flex: 1,
        display: "flex",
        flexDirection: "column",
        minWidth: 0,
        background: "var(--bg-chat)",
        borderLeft: "1px solid var(--border)",
      }}
    >
      <div
        role="tablist"
        style={{
          display: "flex",
          background: "var(--bg-side-header)",
          borderBottom: "1px solid var(--border)",
          height: 34,
          flexShrink: 0,
        }}
      >
        <TabButton
          label={t("agent_tab_label")}
          icon={<MessageSquare size={13} strokeWidth={2} />}
          active={active === "agent"}
          onClick={() => switchTo("agent")}
        />
        <TabButton
          label={t("tutor_tab_label")}
          icon={<Lightbulb size={13} strokeWidth={2} color={AMBER_ACCENT} />}
          active={active === "tips"}
          accent={AMBER_ACCENT}
          accentBg={AMBER_TINT_TAB_ACTIVE}
          highlight={tipsHighlighted}
          onClick={() => switchTo("tips")}
        />
      </div>

      {active === "agent" ? (
        <div style={{ flex: 1, display: "flex", flexDirection: "column", minHeight: 0 }}>
          <ChatPanel
            messages={messages}
            onSend={onSend}
            busy={busy}
            onApply={onApply}
            onPatchResolved={onPatchResolved}
            availableFiles={fileNames}
          />
        </div>
      ) : (
        <TipsView
          sessionId={sessionId}
          submitted={submitted}
          fileContents={fileContents}
          availableFiles={fileNames}
          onTipsTouched={markTipsTouched}
        />
      )}
    </div>
  );
}

function TabButton({
  label,
  icon,
  active,
  onClick,
  accent,
  accentBg,
  highlight,
}: {
  label: string;
  icon: React.ReactNode;
  active: boolean;
  onClick: () => void;
  accent?: string;
  accentBg?: string;
  highlight?: boolean;
}) {
  const indicator = accent ?? "var(--accent)";
  return (
    <button
      type="button"
      role="tab"
      aria-selected={active}
      onClick={onClick}
      style={{
        display: "flex",
        alignItems: "center",
        gap: 6,
        padding: "0 14px",
        fontSize: 12.5,
        color: active ? "var(--text-strong)" : "var(--text-dim)",
        background: active ? accentBg ?? "var(--bg-tab-active)" : "transparent",
        borderRight: "1px solid var(--border)",
        borderBottom: active ? `2px solid ${indicator}` : "2px solid transparent",
        cursor: "pointer",
        userSelect: "none",
        fontWeight: active ? 600 : 500,
      }}
    >
      {icon}
      {label}
      {highlight && !active && (
        <span
          aria-hidden
          style={{
            width: 6,
            height: 6,
            borderRadius: "50%",
            background: indicator,
            marginLeft: 2,
            boxShadow: `0 0 0 2px ${indicator}33`,
          }}
        />
      )}
    </button>
  );
}
