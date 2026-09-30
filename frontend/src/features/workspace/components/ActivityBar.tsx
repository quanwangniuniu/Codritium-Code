"use client";

import { Files, Search, GitBranch, Play } from "lucide-react";
import type { ComponentType } from "react";

export type ActivityView = "files" | "search" | "git" | "run";

interface Item {
  id: ActivityView;
  Icon: ComponentType<{ size?: number; strokeWidth?: number }>;
  label: string;
}

export function ActivityBar({
  active,
  onChange,
}: {
  active: ActivityView;
  onChange: (v: ActivityView) => void;
}) {
  const items: Item[] = [
    { id: "files", Icon: Files, label: "Explorer" },
    { id: "search", Icon: Search, label: "Search" },
    { id: "git", Icon: GitBranch, label: "Source Control" },
    { id: "run", Icon: Play, label: "Run" },
  ];

  return (
    <nav
      style={{
        width: "var(--activity-width)",
        background: "var(--bg-activity)",
        borderRight: "1px solid var(--border)",
        display: "flex",
        flexDirection: "column",
        alignItems: "stretch",
        padding: "4px 0",
        flexShrink: 0,
      }}
    >
      {items.map((it) => {
        const isActive = active === it.id;
        return (
          <button
            key={it.id}
            onClick={() => onChange(it.id)}
            title={it.label}
            style={{
              width: "var(--activity-width)",
              height: 48,
              color: isActive ? "var(--icon-activity-active)" : "var(--icon-activity)",
              borderLeft: isActive
                ? "2px solid var(--icon-activity-active)"
                : "2px solid transparent",
              background: "transparent",
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              cursor: "pointer",
              transition: "color 80ms linear",
            }}
            onMouseEnter={(e) => {
              if (!isActive) e.currentTarget.style.color = "var(--icon-activity-active)";
            }}
            onMouseLeave={(e) => {
              if (!isActive)
                e.currentTarget.style.color = "var(--icon-activity)";
            }}
          >
            <it.Icon size={22} strokeWidth={1.5} />
          </button>
        );
      })}
    </nav>
  );
}
