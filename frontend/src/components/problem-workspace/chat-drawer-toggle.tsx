"use client";

import { useState } from "react";
import { Sparkles, X } from "lucide-react";
import { ChatPlaceholder } from "./chat-placeholder";
import { t } from "@/lib/i18n";
import { useLocale } from "@/lib/i18n-client";

interface ChatDrawerToggleProps {
  problemId: string;
}

export function ChatDrawerToggle({ problemId }: ChatDrawerToggleProps) {
  useLocale();
  const [open, setOpen] = useState(false);

  return (
    <>
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        className="absolute bottom-12 right-4 z-20 inline-flex items-center gap-1.5 rounded-full bg-accent text-accent-fg px-3 py-2 text-xs font-mono shadow-lg hover:opacity-90 transition-opacity"
        aria-label={open ? t("chat_drawer_close_label") : t("chat_drawer_open_label")}
      >
        {open ? <X size={12} /> : <Sparkles size={12} />}
        {open ? t("chat_drawer_close_text") : t("chat_drawer_agent_text")}
      </button>

      {open && (
        <aside className="absolute bottom-12 right-4 top-4 w-80 z-10 border border-divider rounded-md bg-surface shadow-2xl overflow-hidden flex flex-col">
          <ChatPlaceholder variant="drawer" problemId={problemId} />
        </aside>
      )}
    </>
  );
}
