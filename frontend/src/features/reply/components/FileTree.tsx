"use client";

import { motion } from "framer-motion";
import { File, FileText } from "lucide-react";
import { t } from "@/shared/i18n";
import { useLocale } from "@/shared/i18n/client";

// FileTree is the IDE-style sidebar on the left column of the replay UI.
// The active file slides via a shared layout id, modified files carry a
// muted dot, and selection is a controlled callback so the parent can
// route both manual clicks and replay-driven highlights through the same
// state.

export interface FileTreeProps {
  files: string[];
  activeFile: string | null;
  onSelect: (name: string) => void;
  modifiedFiles?: Set<string>;
}

export function FileTree({
  files,
  activeFile,
  onSelect,
  modifiedFiles,
}: FileTreeProps) {
  useLocale();
  return (
    <div
      style={{
        display: "flex",
        flexDirection: "column",
        gap: 1,
        padding: "8px 0",
      }}
    >
      <div
        style={{
          fontSize: 10.5,
          textTransform: "uppercase",
          letterSpacing: 0.6,
          fontWeight: 600,
          color: "var(--muted)",
          padding: "4px 12px 8px",
        }}
      >
        {t("filetree_files_header")}
      </div>
      {files.map((name) => {
        const active = name === activeFile;
        const modified = modifiedFiles?.has(name) ?? false;
        return (
          <button
            key={name}
            type="button"
            onClick={() => onSelect(name)}
            style={{
              position: "relative",
              display: "flex",
              alignItems: "center",
              gap: 8,
              padding: "5px 12px",
              border: "none",
              background: "transparent",
              cursor: "pointer",
              textAlign: "left",
              fontSize: 12.5,
              color: active ? "var(--ink)" : "var(--muted)",
              fontWeight: active ? 600 : 500,
            }}
          >
            {active && (
              <motion.div
                layoutId="filetree-active"
                style={{
                  position: "absolute",
                  inset: 0,
                  background: "var(--accent-soft, rgba(59, 130, 246, 0.12))",
                  borderLeft: "2px solid var(--accent, #3b82f6)",
                  zIndex: 0,
                }}
                transition={{ type: "spring", stiffness: 380, damping: 32 }}
              />
            )}
            <span style={{ position: "relative", zIndex: 1, display: "inline-flex", alignItems: "center", gap: 8 }}>
              {extensionOf(name) === "py" ? (
                <FileText size={13} strokeWidth={1.8} />
              ) : (
                <File size={13} strokeWidth={1.8} />
              )}
              {name}
            </span>
            {modified && (
              <span
                aria-label={t("filetree_modified_label")}
                style={{
                  position: "relative",
                  zIndex: 1,
                  marginLeft: "auto",
                  width: 6,
                  height: 6,
                  borderRadius: 999,
                  background: "var(--accent, #3b82f6)",
                }}
              />
            )}
          </button>
        );
      })}
    </div>
  );
}

function extensionOf(name: string): string {
  const dot = name.lastIndexOf(".");
  return dot >= 0 ? name.slice(dot + 1) : "";
}
