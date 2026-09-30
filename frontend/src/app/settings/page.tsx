"use client";

import { useRouter } from "next/navigation";
import { ChevronLeft, Layout, Type, Lightbulb } from "lucide-react";
import { useRequireAuth } from "@/hooks/useRequireAuth";
import { t } from "@/shared/i18n";

// Settings is a placeholder scaffold during v0.9. The intent (per the
// 2026-05-20 tips-frontend interv §20) is to give the workspace a place
// to grow into: layout toggle (Agent + Tutor tabs vs side-by-side), font
// preferences for the Monaco editor, and Tutor visibility. Each section
// renders as a disabled card with "Coming soon" copy until the backing
// preference store and per-section UI land.
export default function SettingsPage() {
  const me = useRequireAuth();
  const router = useRouter();
  if (!me) {
    return null;
  }

  return (
    <main
      style={{
        maxWidth: 720,
        margin: "0 auto",
        padding: "40px 24px 80px",
        color: "var(--ink)",
      }}
    >
      <button
        onClick={() => router.back()}
        style={{
          display: "inline-flex",
          alignItems: "center",
          gap: 4,
          padding: "4px 8px",
          fontSize: 12,
          color: "var(--muted)",
          background: "transparent",
          border: "1px solid var(--divider)",
          borderRadius: 4,
          cursor: "pointer",
          marginBottom: 24,
        }}
      >
        <ChevronLeft size={14} />
        Back
      </button>

      <h1 style={{ fontSize: 26, fontWeight: 600, marginBottom: 6 }}>
        {t("settings_title")}
      </h1>
      <p style={{ fontSize: 14, color: "var(--muted)", marginBottom: 28 }}>
        {t("settings_subtitle")}
      </p>

      <div style={{ display: "flex", flexDirection: "column", gap: 16 }}>
        <SettingsCard
          icon={<Layout size={18} strokeWidth={1.7} />}
          title={t("settings_layout_section")}
          hint={t("settings_layout_hint")}
        />
        <SettingsCard
          icon={<Type size={18} strokeWidth={1.7} />}
          title={t("settings_font_section")}
          hint={t("settings_font_hint")}
        />
        <SettingsCard
          icon={<Lightbulb size={18} strokeWidth={1.7} color="#f59e0b" />}
          title={t("settings_tips_visibility_section")}
          hint={t("settings_tips_visibility_hint")}
        />
      </div>
    </main>
  );
}

function SettingsCard({
  icon,
  title,
  hint,
}: {
  icon: React.ReactNode;
  title: string;
  hint: string;
}) {
  return (
    <div
      style={{
        border: "1px solid var(--divider)",
        borderRadius: 8,
        padding: "16px 18px",
        background: "var(--surface)",
        display: "flex",
        gap: 14,
        opacity: 0.85,
      }}
    >
      <div
        style={{
          width: 36,
          height: 36,
          borderRadius: 8,
          background: "var(--surface-2)",
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          color: "var(--muted)",
          flexShrink: 0,
        }}
      >
        {icon}
      </div>
      <div style={{ flex: 1 }}>
        <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
          <h2 style={{ fontSize: 15, fontWeight: 600 }}>{title}</h2>
          <span
            style={{
              fontSize: 10.5,
              padding: "2px 6px",
              borderRadius: 999,
              background: "var(--surface-2)",
              color: "var(--muted)",
              textTransform: "uppercase",
              letterSpacing: 0.4,
              fontWeight: 600,
            }}
          >
            {t("settings_coming_soon")}
          </span>
        </div>
        <p style={{ fontSize: 12.5, color: "var(--muted)", marginTop: 4, lineHeight: 1.5 }}>
          {hint}
        </p>
      </div>
    </div>
  );
}
