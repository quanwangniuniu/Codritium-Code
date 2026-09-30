"use client";

import Link from "next/link";
import { Clock, BookOpen, HelpCircle, Play } from "lucide-react";
import { CodritiumLogo } from "@/components/CodritiumLogo";
import { ExitWorkspaceButton } from "@/components/ExitWorkspaceButton";
import { ThemeToggle } from "@/components/ThemeToggle";
import { t } from "@/lib/i18n";
import { useLocale } from "@/lib/i18n-client";

export function AppBar({
  timer,
  onSubmit,
  problemTitle,
  submitting,
  submitDisabled,
}: {
  timer: string;
  onSubmit: () => void;
  problemTitle: string;
  submitting: boolean;
  submitDisabled: boolean;
}) {
  useLocale();
  return (
    <header
      style={{
        height: "var(--appbar-height)",
        background: "var(--bg-side-header)",
        borderBottom: "1px solid var(--border-strong)",
        display: "flex",
        alignItems: "center",
        padding: "0 12px",
        gap: 12,
        flexShrink: 0,
        color: "var(--text)",
      }}
    >
      <Link
        href="/"
        aria-label={t("brand_name")}
        style={{
          display: "flex",
          alignItems: "center",
          color: "var(--text-strong)",
          textDecoration: "none",
        }}
      >
        <CodritiumLogo iconOnly height={18} />
      </Link>

      <ExitWorkspaceButton />

      <div
        style={{
          fontSize: 12,
          color: "var(--text-dim)",
          paddingLeft: 12,
          borderLeft: "1px solid var(--border)",
        }}
      >
        {problemTitle}
      </div>

      <div style={{ flex: 1 }} />

      <div
        className="mono"
        style={{
          fontSize: 12,
          color: "var(--text)",
          padding: "3px 9px",
          background: "var(--bg-tab)",
          borderRadius: 4,
          border: "1px solid var(--border)",
          display: "inline-flex",
          alignItems: "center",
          gap: 5,
        }}
      >
        <Clock size={12} strokeWidth={1.7} style={{ color: "var(--text-dim)" }} />
        <span>{timer}</span>
      </div>

      <AppBarButton icon={<BookOpen size={13} strokeWidth={1.7} />} label={t("appbar_readme")} />
      <AppBarButton icon={<HelpCircle size={13} strokeWidth={1.7} />} label={t("appbar_help")} />

      <ThemeToggle />

      <button
        onClick={onSubmit}
        disabled={submitDisabled}
        aria-busy={submitting}
        style={{
          fontSize: 12,
          color: "#ffffff",
          padding: "5px 14px",
          background: "var(--accent)",
          borderRadius: 4,
          fontWeight: 600,
          display: "inline-flex",
          alignItems: "center",
          gap: 6,
          cursor: submitDisabled ? "not-allowed" : "pointer",
          opacity: submitDisabled ? 0.65 : 1,
        }}
        onMouseEnter={(e) => {
          e.currentTarget.style.background = "var(--accent-hover)";
        }}
        onMouseLeave={(e) => {
          e.currentTarget.style.background = "var(--accent)";
        }}
      >
        <Play size={12} strokeWidth={2} />
        <span>{submitting ? "Submitting…" : t("submit")}</span>
      </button>
    </header>
  );
}

function AppBarButton({ icon, label }: { icon: React.ReactNode; label: string }) {
  return (
    <button
      style={{
        fontSize: 12,
        color: "var(--text)",
        padding: "5px 9px",
        display: "inline-flex",
        alignItems: "center",
        gap: 5,
        borderRadius: 3,
        background: "transparent",
        border: "none",
        cursor: "pointer",
      }}
      onMouseEnter={(e) => {
        e.currentTarget.style.background = "var(--bg-tab-hover)";
      }}
      onMouseLeave={(e) => {
        e.currentTarget.style.background = "transparent";
      }}
    >
      {icon}
      <span>{label}</span>
    </button>
  );
}
