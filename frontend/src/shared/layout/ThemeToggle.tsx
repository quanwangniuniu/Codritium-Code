"use client";

import { useEffect, useState } from "react";
import { Moon, Sun } from "lucide-react";

type Theme = "default" | "bright";
const COOKIE = "codritium_theme";

function readTheme(): Theme {
  if (typeof document === "undefined") return "default";
  return document.documentElement.dataset.theme === "bright" ? "bright" : "default";
}

export function ThemeToggle() {
  const [theme, setTheme] = useState<Theme>("default");

  useEffect(() => {
    setTheme(readTheme());
    const observer = new MutationObserver(() => setTheme(readTheme()));
    observer.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ["data-theme"],
    });
    return () => observer.disconnect();
  }, []);

  const isDark = theme === "default";
  const toggle = () => {
    const next: Theme = isDark ? "bright" : "default";
    document.cookie = `${COOKIE}=${next};path=/;max-age=${60 * 60 * 24 * 365};samesite=lax`;
    document.documentElement.dataset.theme = next;
  };

  return (
    <button
      onClick={toggle}
      title={isDark ? "Switch to light theme" : "Switch to dark theme"}
      aria-label={isDark ? "Switch to light theme" : "Switch to dark theme"}
      style={{
        display: "inline-flex",
        alignItems: "center",
        justifyContent: "center",
        width: 28,
        height: 28,
        color: "var(--text)",
        background: "transparent",
        borderRadius: 3,
        cursor: "pointer",
        border: "none",
      }}
      onMouseEnter={(e) => {
        e.currentTarget.style.background = "var(--bg-tab-hover)";
      }}
      onMouseLeave={(e) => {
        e.currentTarget.style.background = "transparent";
      }}
    >
      {isDark ? (
        <Sun size={14} strokeWidth={1.7} />
      ) : (
        <Moon size={14} strokeWidth={1.7} />
      )}
    </button>
  );
}
