"use client";

import { BookOpen, FileCode2, FileText, ChevronDown } from "lucide-react";

export type FileTreeItem = { name: string; type: "file"; selected?: boolean };

function FileIcon({ name }: { name: string }) {
  const isPy = name.endsWith(".py");
  const isTs = name.endsWith(".ts") || name.endsWith(".tsx");
  const isJs = name.endsWith(".js") || name.endsWith(".jsx");
  const isMd = name.endsWith(".md");
  let color = "var(--text-muted)";
  let Icon: typeof FileCode2 = FileCode2;
  if (isPy) color = "var(--icon-python)";
  else if (isTs) color = "#3178c6";
  else if (isJs) color = "#f7df1e";
  else if (isMd) {
    color = "var(--text-muted)";
    Icon = FileText;
  } else {
    Icon = FileText;
  }
  return (
    <span style={{ color, display: "inline-flex", alignItems: "center" }}>
      <Icon size={14} strokeWidth={1.5} />
    </span>
  );
}

export function SideBar({
  files,
  activeFile,
  onSelectFile,
  extraSlot,
}: {
  files: FileTreeItem[];
  activeFile: string | null;
  onSelectFile: (name: string) => void;
  // Optional bottom slot (V0-5c: PendingPatchesPanel docks here).
  extraSlot?: React.ReactNode;
}) {
  return (
    <aside
      style={{
        width: "100%",
        height: "100%",
        background: "var(--bg-side)",
        display: "flex",
        flexDirection: "column",
        flexShrink: 0,
        overflow: "hidden",
      }}
    >
      <div
        style={{
          padding: "9px 18px 8px",
          fontSize: 11,
          textTransform: "uppercase",
          letterSpacing: "0.08em",
          color: "var(--text-dim)",
          fontWeight: 600,
          flexShrink: 0,
        }}
      >
        Explorer
      </div>

      <div style={{ flexShrink: 0 }}>
        <SectionHeader label="Problem" />
        <FileRow
          icon={
            <BookOpen
              size={14}
              strokeWidth={1.5}
              style={{ color: "var(--text-muted)" }}
            />
          }
          name="README.md"
          isActive={activeFile === "README.md"}
          onClick={() => onSelectFile("README.md")}
        />
      </div>

      <div style={{ flexShrink: 0, marginTop: 4 }}>
        <SectionHeader label="Starter Files" />
        {files.map((f) => (
          <FileRow
            key={f.name}
            icon={<FileIcon name={f.name} />}
            name={f.name}
            isActive={activeFile === f.name}
            onClick={() => onSelectFile(f.name)}
          />
        ))}
      </div>

      {extraSlot}

      <div
        style={{
          padding: "6px 18px",
          fontSize: 10,
          color: "var(--text-muted)",
          borderTop: "1px solid var(--border-soft)",
          flexShrink: 0,
        }}
      >
        Codritium MVP · v0.8
      </div>
    </aside>
  );
}

function SectionHeader({ label }: { label: string }) {
  return (
    <div
      style={{
        display: "flex",
        alignItems: "center",
        gap: 4,
        padding: "4px 14px 2px",
        fontSize: 10.5,
        color: "var(--text-dim)",
        textTransform: "uppercase",
        letterSpacing: "0.06em",
        fontWeight: 600,
      }}
    >
      <ChevronDown size={11} strokeWidth={2} style={{ color: "var(--text-muted)" }} />
      <span>{label}</span>
    </div>
  );
}

function FileRow({
  icon,
  name,
  isActive,
  onClick,
}: {
  icon: React.ReactNode;
  name: string;
  isActive: boolean;
  onClick: () => void;
}) {
  return (
    <button
      onClick={onClick}
      style={{
        display: "flex",
        alignItems: "center",
        gap: 6,
        padding: "3px 18px 3px 26px",
        width: "100%",
        fontSize: 13,
        color: isActive ? "var(--text-strong)" : "var(--text)",
        background: isActive ? "var(--bg-row-active)" : "transparent",
        cursor: "pointer",
        textAlign: "left",
      }}
      onMouseEnter={(e) => {
        if (!isActive) e.currentTarget.style.background = "var(--bg-row-hover)";
      }}
      onMouseLeave={(e) => {
        if (!isActive) e.currentTarget.style.background = "transparent";
      }}
    >
      {icon}
      <span>{name}</span>
    </button>
  );
}
