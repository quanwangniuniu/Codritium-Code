"use client";

import type { Me } from "@/lib/api";

export function StatusBar({
  me,
  problemSlug,
  language,
  cursorLine,
  cursorCol,
}: {
  me: Me | null;
  problemSlug: string;
  language: string;
  cursorLine: number;
  cursorCol: number;
}) {
  return (
    <footer
      style={{
        height: "var(--statusbar-height)",
        background: "var(--bg-side)",
        borderTop: "1px solid var(--border)",
        display: "flex",
        alignItems: "center",
        padding: "0 10px",
        fontSize: 11,
        color: "var(--text-dim)",
        gap: 14,
        flexShrink: 0,
      }}
    >
      <span>⊗ 0  ⚠ 0</span>
      <span style={{ flex: 1 }} />
      <span className="mono">
        {me ? `@${me.handle}` : "—"} · {problemSlug}
      </span>
      <span className="mono">
        Ln {cursorLine}, Col {cursorCol}
      </span>
      <span>Spaces: 4</span>
      <span>UTF-8</span>
      <span>LF</span>
      <span>{ "{}" } {language}</span>
    </footer>
  );
}
