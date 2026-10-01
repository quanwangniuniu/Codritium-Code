"use client";

import { useEffect, useState } from "react";
import { subscribeBanner, type BannerState } from "@/shared/lib/banner";

const VARIANT_STYLE: Record<
  "warn" | "error",
  { background: string; border: string; color: string; icon: string }
> = {
  warn: {
    background: "rgba(252, 211, 77, 0.10)",
    border: "rgba(252, 211, 77, 0.40)",
    color: "#fcd34d",
    icon: "!",
  },
  error: {
    background: "rgba(252, 165, 165, 0.10)",
    border: "rgba(252, 165, 165, 0.50)",
    color: "#fca5a5",
    icon: "✕",
  },
};

export function Banner() {
  const [state, setState] = useState<BannerState>({
    visible: false,
    variant: "warn",
    message: "",
  });

  useEffect(() => {
    return subscribeBanner(setState);
  }, []);

  if (!state.visible) return null;
  const v = VARIANT_STYLE[state.variant];
  return (
    <div
      role="status"
      aria-live="polite"
      style={{
        position: "sticky",
        top: 0,
        zIndex: 9998,
        background: v.background,
        borderBottom: `1px solid ${v.border}`,
        color: v.color,
        padding: "8px 16px",
        fontSize: 12.5,
        display: "flex",
        alignItems: "center",
        gap: 10,
      }}
    >
      <span aria-hidden="true" style={{ fontWeight: 700 }}>
        {v.icon}
      </span>
      <span style={{ flex: 1 }}>{state.message}</span>
    </div>
  );
}
