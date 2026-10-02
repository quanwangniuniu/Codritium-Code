"use client";

import { DiffEditor } from "@monaco-editor/react";

import { Code2, SplitSquareHorizontal, X, XSquare } from "lucide-react";
import { IDEPane } from "@/shared/editor/IDEPane";
import { useMonacoTheme } from "@/shared/editor/useMonacoTheme";
import { languageForFile } from "@/shared/editor/language";
import { ReadmeTab } from "@/features/workspace/components/ReadmeTab";
import type { EditorDiffPreview, OpenTab, } from "@/features/workspace/types";

export const README_TAB = "README.md";

export interface EditorPaneProps {
  paneId: string;
  tabs: OpenTab[];
  activeTab: string | null;
  fileContents: Record<string, string>;
  diffPreview?: EditorDiffPreview | null;
  onDiffPreviewChange?: (toolUseId: string, content: string) => void;
  readmeMD?: string;
  readmeFloating?: boolean;
  onToggleReadmeFloat?: () => void;
  onActivateTab: (name: string) => void;
  onCloseTab: (name: string) => void;
  onContentChange: (name: string, value: string) => void;
  onSplitRight?: () => void;
  onClosePane?: () => void;
  canSplit?: boolean;
}

export function EditorPane({
  paneId,
  tabs,
  activeTab,
  fileContents,
  diffPreview,
  onDiffPreviewChange,
  readmeMD = "",
  readmeFloating = false,
  onToggleReadmeFloat,
  onActivateTab,
  onCloseTab,
  onContentChange,
  onSplitRight,
  onClosePane,
  canSplit = false,
}: EditorPaneProps) {
  const monacoTheme = useMonacoTheme();
  const hasActive = activeTab !== null;
  const isReadme = activeTab === README_TAB;
  const activeContent = activeTab ? fileContents[activeTab] ?? "" : "";
  const activeDiff = activeTab && diffPreview?.path === activeTab ? diffPreview : null;

  return (
    <section
      data-pane-id={paneId}
      style={{
        flex: 1,
        display: "flex",
        flexDirection: "column",
        minWidth: 0,
        minHeight: 0,
        background: "var(--bg-editor)",
        height: "100%",
      }}
    >
      <div
        style={{
          display: "flex",
          background: "var(--bg-side)",
          borderBottom: "1px solid var(--border)",
          height: 35,
          flexShrink: 0,
          alignItems: "stretch",
        }}
      >
        <div
          style={{
            display: "flex",
            overflowX: "auto",
            flex: 1,
            minWidth: 0,
          }}
        >
          {tabs.length === 0 && (
            <div
              style={{
                padding: "0 12px",
                fontSize: 12,
                color: "var(--text-muted)",
                display: "flex",
                alignItems: "center",
              }}
            >
              No file open
            </div>
          )}
          {tabs.map((tab) => {
            const isActive = activeTab === tab.name;
            return (
              <div
                key={tab.name}
                onClick={() => onActivateTab(tab.name)}
                style={{
                  display: "flex",
                  alignItems: "center",
                  gap: 6,
                  padding: "0 10px 0 12px",
                  fontSize: 12.5,
                  color: isActive ? "var(--text-strong)" : "var(--text-dim)",
                  background: isActive ? "var(--bg-editor)" : "transparent",
                  borderRight: "1px solid var(--border)",
                  borderTop: isActive
                    ? "1px solid var(--accent)"
                    : "1px solid transparent",
                  cursor: "pointer",
                  whiteSpace: "nowrap",
                  position: "relative",
                  minWidth: 0,
                }}
              >
                <span className="mono">{tab.name}</span>
                {tab.dirty && (
                  <span
                    style={{
                      width: 7,
                      height: 7,
                      borderRadius: "50%",
                      background: "var(--text-strong)",
                    }}
                  />
                )}
                <button
                  onClick={(e) => {
                    e.stopPropagation();
                    onCloseTab(tab.name);
                  }}
                  style={{
                    color: "var(--text-dim)",
                    padding: 2,
                    marginLeft: 2,
                    display: "flex",
                    alignItems: "center",
                    borderRadius: 3,
                  }}
                  onMouseEnter={(e) => {
                    e.currentTarget.style.background = "var(--bg-tab-hover)";
                  }}
                  onMouseLeave={(e) => {
                    e.currentTarget.style.background = "transparent";
                  }}
                  title="Close"
                >
                  <X size={13} strokeWidth={1.7} />
                </button>
              </div>
            );
          })}
        </div>

        <div
          style={{
            display: "flex",
            alignItems: "center",
            gap: 2,
            paddingRight: 6,
            paddingLeft: 6,
            borderLeft: "1px solid var(--border)",
            flexShrink: 0,
          }}
        >
          {canSplit && onSplitRight && (
            <button
              onClick={onSplitRight}
              title="Split editor right"
              style={paneActionBtnStyle}
              onMouseEnter={(e) => {
                e.currentTarget.style.background = "var(--bg-tab-hover)";
              }}
              onMouseLeave={(e) => {
                e.currentTarget.style.background = "transparent";
              }}
            >
              <SplitSquareHorizontal size={14} strokeWidth={1.6} />
            </button>
          )}
          {onClosePane && (
            <button
              onClick={onClosePane}
              title="Close editor pane"
              style={paneActionBtnStyle}
              onMouseEnter={(e) => {
                e.currentTarget.style.background = "var(--bg-tab-hover)";
              }}
              onMouseLeave={(e) => {
                e.currentTarget.style.background = "transparent";
              }}
            >
              <XSquare size={14} strokeWidth={1.6} />
            </button>
          )}
        </div>
      </div>

      <div style={{ flex: 1, minHeight: 0, position: "relative" }}>
        {hasActive && isReadme ? (
          <ReadmeTab
            readme={readmeMD}
            floating={readmeFloating}
            onToggleFloat={onToggleReadmeFloat ?? (() => undefined)}
          />
        ) : hasActive && activeDiff ? (
          <DiffEditor
            height="100%"
            width="100%"
            language={languageForFile(activeTab)}
            original={activeDiff.original}
            modified={activeDiff.modified}
            theme={monacoTheme}
            onMount={(editor) => {
              const modifiedEditor = editor.getModifiedEditor();

              modifiedEditor.onDidChangeModelContent(() => {
                onDiffPreviewChange?.(
                  activeDiff.toolUseId,
                  modifiedEditor.getValue(),
                );
              });
            }}
            options={{
              automaticLayout: true,
              readOnly: false,
              originalEditable: false,
              renderSideBySide: false,
              renderOverviewRuler: true,
              minimap: { enabled: false },
              stickyScroll: { enabled: false },
              scrollBeyondLastLine: false,
              fontSize: 13,
            }}
          />
        ) : hasActive ? (
          <IDEPane
            path={activeTab}
            language={languageForFile(activeTab)}
            value={activeContent}
            onChange={(v) => onContentChange(activeTab, v)}
            theme={monacoTheme}
          />
        ) : (
          <div
            style={{
              height: "100%",
              display: "flex",
              flexDirection: "column",
              alignItems: "center",
              justifyContent: "center",
              gap: 14,
              color: "var(--text-muted)",
              fontSize: 13,
              userSelect: "none",
            }}
          >
            <Code2 size={56} strokeWidth={1} style={{ opacity: 0.35 }} />
            <div
              style={{
                fontSize: 13,
                fontWeight: 500,
                color: "var(--text-dim)",
              }}
            >
              Codritium Editor
            </div>
            <div
              style={{
                fontSize: 11.5,
                textAlign: "center",
                lineHeight: 1.7,
                maxWidth: 320,
              }}
            >
              Pick a file from the sidebar to start editing.
            </div>
          </div>
        )}
      </div>
    </section>
  );
}

const paneActionBtnStyle: React.CSSProperties = {
  display: "flex",
  alignItems: "center",
  justifyContent: "center",
  width: 24,
  height: 24,
  color: "var(--text-dim)",
  background: "transparent",
  border: "none",
  borderRadius: 3,
  cursor: "pointer",
};
