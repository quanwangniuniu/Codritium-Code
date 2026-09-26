"use client";

import { PanelLayout, type PaneDescriptor } from "./PanelLayout";
import { EditorPane } from "./EditorPane";
import type { OpenTab } from "@/lib/types";

export interface EditorPaneState {
  id: string;
  tabs: OpenTab[];
  activeTab: string | null;
}

export interface EditorAreaSplitProps {
  panes: EditorPaneState[];
  fileContents: Record<string, string>;
  readmeMD?: string;
  readmeFloating?: boolean;
  onToggleReadmeFloat?: () => void;
  onActivateTab: (paneId: string, name: string) => void;
  onCloseTab: (paneId: string, name: string) => void;
  onContentChange: (paneId: string, name: string, value: string) => void;
  onSplitFrom: (paneId: string) => void;
  onClosePane: (paneId: string) => void;
  canSplit: boolean;
  onLayoutChanged?: (sizes: number[]) => void;
}

export function EditorAreaSplit({
  panes,
  fileContents,
  readmeMD,
  readmeFloating,
  onToggleReadmeFloat,
  onActivateTab,
  onCloseTab,
  onContentChange,
  onSplitFrom,
  onClosePane,
  canSplit,
  onLayoutChanged,
}: EditorAreaSplitProps) {
  if (panes.length === 0) return null;
  const descriptors: PaneDescriptor[] = panes.map((pane) => ({
    id: pane.id,
    defaultSize: 100 / panes.length,
    minSize: 15,
    content: (
      <EditorPane
        paneId={pane.id}
        tabs={pane.tabs}
        activeTab={pane.activeTab}
        fileContents={fileContents}
        readmeMD={readmeMD}
        readmeFloating={readmeFloating}
        onToggleReadmeFloat={onToggleReadmeFloat}
        onActivateTab={(name) => onActivateTab(pane.id, name)}
        onCloseTab={(name) => onCloseTab(pane.id, name)}
        onContentChange={(name, value) =>
          onContentChange(pane.id, name, value)
        }
        onSplitRight={() => onSplitFrom(pane.id)}
        onClosePane={panes.length > 1 ? () => onClosePane(pane.id) : undefined}
        canSplit={canSplit}
      />
    ),
  }));

  return (
    <PanelLayout
      panes={descriptors}
      direction="horizontal"
      onLayoutChanged={onLayoutChanged}
    />
  );
}
