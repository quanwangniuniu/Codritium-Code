"use client";

import { dismiss, type ToastItem } from "@/lib/toast";
import { t } from "@/lib/i18n";

const VARIANT_STYLE: Record<
  ToastItem["variant"],
  { background: string; border: string; accent: string; icon: string }
> = {
  info: {
    background: "rgba(125, 211, 252, 0.10)",
    border: "rgba(125, 211, 252, 0.40)",
    accent: "#7dd3fc",
    icon: "ⓘ",
  },
  success: {
    background: "rgba(134, 239, 172, 0.10)",
    border: "rgba(134, 239, 172, 0.40)",
    accent: "#86efac",
    icon: "✓",
  },
  warn: {
    background: "rgba(252, 211, 77, 0.10)",
    border: "rgba(252, 211, 77, 0.40)",
    accent: "#fcd34d",
    icon: "!",
  },
  error: {
    background: "rgba(252, 165, 165, 0.10)",
    border: "rgba(252, 165, 165, 0.50)",
    accent: "#fca5a5",
    icon: "✕",
  },
};

export function Toast({ item }: { item: ToastItem }) {
  const v = VARIANT_STYLE[item.variant];
  return (
    <div
      role="status"
      aria-live={item.variant === "error" ? "assertive" : "polite"}
      style={{
        display: "flex",
        alignItems: "flex-start",
        gap: 10,
        background: "#181b22",
        border: `1px solid ${v.border}`,
        borderLeft: `3px solid ${v.accent}`,
        borderRadius: 6,
        padding: "10px 12px",
        boxShadow: "0 4px 12px rgba(0, 0, 0, 0.4)",
        minWidth: 280,
        maxWidth: 420,
        fontSize: 13,
        lineHeight: 1.55,
        color: "#e6e8ee",
        animation: "toast-slide-in 0.18s ease-out",
      }}
    >
      <span
        aria-hidden="true"
        style={{
          color: v.accent,
          fontWeight: 700,
          fontSize: 14,
          lineHeight: 1.4,
          flexShrink: 0,
          minWidth: 14,
        }}
      >
        {v.icon}
      </span>
      <span style={{ flex: 1, wordBreak: "break-word" }}>{item.message}</span>
      <button
        type="button"
        aria-label={t("close")}
        onClick={() => dismiss(item.id)}
        style={{
          background: "transparent",
          border: 0,
          color: "#6b7280",
          cursor: "pointer",
          padding: 0,
          marginLeft: 4,
          fontSize: 16,
          lineHeight: 1,
          flexShrink: 0,
        }}
      >
        ×
      </button>
    </div>
  );
}
