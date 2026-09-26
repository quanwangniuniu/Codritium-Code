"use client";

import { useMemo } from "react";

// Q1=B per ide-multi-pane impl-spec: 4 panes is the V1.0 conservative
// upper bound (Monaco profile reality + VS Code's own "5 tabs, 2 editors"
// heuristic). Raise once a Chrome devtools profile says so.
export const MAX_PANES = 4;

export interface PaneLimitState {
  count: number;
  max: number;
  canAddPane: boolean;
}

export function usePaneLimit(count: number, max: number = MAX_PANES): PaneLimitState {
  return useMemo(
    () => ({ count, max, canAddPane: count < max }),
    [count, max],
  );
}
