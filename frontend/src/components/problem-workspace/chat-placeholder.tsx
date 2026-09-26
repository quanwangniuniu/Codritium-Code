"use client";

import { useState } from "react";
import { Send, Sparkles } from "lucide-react";
import { t, type LocaleKey } from "@/lib/i18n";
import { useLocale } from "@/lib/i18n-client";

interface ChatPlaceholderProps {
  variant?: "panel" | "drawer" | "terminal";
  problemId: string;
}

const SEED_BLOCKS: { role: "system" | "user" | "agent"; key: LocaleKey }[] = [
  { role: "system", key: "agent_placeholder_system" },
  { role: "user", key: "agent_placeholder_user" },
  { role: "agent", key: "agent_placeholder_agent" },
];

export function ChatPlaceholder({ variant = "panel", problemId }: ChatPlaceholderProps) {
  useLocale();
  const [draft, setDraft] = useState("");

  const isTerminal = variant === "terminal";

  return (
    <div className="flex flex-col h-full min-h-0 bg-surface border-divider">
      <div className="flex items-center gap-2 px-3 py-2 border-b border-divider flex-shrink-0">
        <Sparkles size={12} className="text-success" />
        <span className="text-xs font-medium text-ink">{t("agent_header")}</span>
        <span className="text-[10px] text-faint truncate">{t("agent_prob_prefix")}{problemId}</span>
      </div>

      <div className="flex-1 min-h-0 overflow-y-auto px-3 py-3 space-y-3 text-xs leading-relaxed">
        {SEED_BLOCKS.map((b, i) => (
          <div key={i} className={isTerminal ? "" : ""}>
            <div className="flex items-baseline gap-2 mb-0.5">
              <span
                className={
                  b.role === "system"
                    ? "text-faint font-mono"
                    : b.role === "user"
                      ? "text-success font-mono"
                      : "text-accent font-mono"
                }
              >
                {b.role === "system" ? "—" : b.role === "user" ? "❯" : "•"}
              </span>
              <span className="text-[10px] text-faint uppercase tracking-wider">
                {b.role}
              </span>
            </div>
            <div
              className={
                isTerminal
                  ? "pl-4 border-l-2 border-divider font-mono text-ink/90"
                  : "pl-4 text-ink/90"
              }
            >
              {t(b.key)}
            </div>
          </div>
        ))}
      </div>

      <form
        onSubmit={(e) => {
          e.preventDefault();
          alert(t("agent_alert_v06"));
        }}
        className="flex items-center gap-2 px-3 py-2 border-t border-divider bg-surface-2 flex-shrink-0"
      >
        <span className="text-success font-mono text-xs">❯</span>
        <input
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          placeholder={t("agent_placeholder_input")}
          className="flex-1 bg-transparent text-xs outline-none placeholder:text-faint font-mono text-ink"
        />
        <button
          type="submit"
          className="text-faint hover:text-ink"
          aria-label={t("send_aria")}
        >
          <Send size={12} />
        </button>
      </form>
    </div>
  );
}
