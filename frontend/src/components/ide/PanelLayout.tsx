"use client";

import { Fragment, ReactNode } from "react";
import {
  Panel,
  PanelGroup,
  PanelResizeHandle,
  type ImperativePanelGroupHandle,
} from "react-resizable-panels";

// Q4=A from the ide-multi-pane impl-spec: a single global autoSaveId keeps
// pane sizes consistent across mounts and route changes. Per-problem
// layouts are an upgrade path for V1.1.
export const IDE_LAYOUT_AUTOSAVE_ID = "codritium-ide";

export interface PaneDescriptor {
  id: string;
  defaultSize: number;
  minSize?: number;
  content: ReactNode;
  onResize?: (size: number) => void;
}

export interface PanelLayoutProps {
  panes: PaneDescriptor[];
  direction?: "horizontal" | "vertical";
  groupRef?: React.Ref<ImperativePanelGroupHandle>;
  onLayoutChanged?: (sizes: number[]) => void;
  autoSaveId?: string;
}

// PanelLayout wraps a PanelGroup with N Panels and resize handles between
// them. Single global autoSaveId by default (Q4=A); callers can override
// per-context when an upgrade path is taken.
export function PanelLayout({
  panes,
  direction = "horizontal",
  groupRef,
  onLayoutChanged,
  autoSaveId = IDE_LAYOUT_AUTOSAVE_ID,
}: PanelLayoutProps) {
  if (panes.length === 0) return null;
  return (
    <PanelGroup
      ref={groupRef}
      direction={direction}
      autoSaveId={autoSaveId}
      onLayout={onLayoutChanged}
    >
      {panes.map((pane, idx) => (
        <Fragment key={pane.id}>
          <Panel
            id={pane.id}
            defaultSize={pane.defaultSize}
            minSize={pane.minSize ?? 10}
            onResize={pane.onResize}
          >
            {pane.content}
          </Panel>
          {idx < panes.length - 1 && (
            <PanelResizeHandle className="ide-resize-handle" />
          )}
        </Fragment>
      ))}
    </PanelGroup>
  );
}
