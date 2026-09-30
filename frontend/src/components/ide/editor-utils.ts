"use client";

import { useEffect, useState } from "react";

export function langFor(filename: string): string {
  if (filename.endsWith(".py")) return "python";
  if (filename.endsWith(".ts") || filename.endsWith(".tsx")) return "typescript";
  if (filename.endsWith(".js") || filename.endsWith(".jsx")) return "javascript";
  if (filename.endsWith(".go")) return "go";
  if (filename.endsWith(".md")) return "markdown";
  return "plaintext";
}

function resolveMonacoTheme(): "vs" | "vs-dark" {
  if (typeof document === "undefined") return "vs-dark";
  const dt = document.documentElement.getAttribute("data-theme");
  if (dt === "bright") return "vs";
  if (typeof window !== "undefined" && window.matchMedia) {
    return window.matchMedia("(prefers-color-scheme: dark)").matches
      ? "vs-dark"
      : "vs";
  }
  return "vs-dark";
}

export function useMonacoTheme(): "vs" | "vs-dark" {
  const [theme, setTheme] = useState<"vs" | "vs-dark">("vs-dark");
  useEffect(() => {
    const update = () => setTheme(resolveMonacoTheme());
    update();
    const observer = new MutationObserver(update);
    observer.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ["data-theme"],
    });
    const mq = window.matchMedia("(prefers-color-scheme: dark)");
    mq.addEventListener("change", update);
    return () => {
      observer.disconnect();
      mq.removeEventListener("change", update);
    };
  }, []);
  return theme;
}
