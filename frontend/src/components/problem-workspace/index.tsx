import type { LayoutPreset } from "@/lib/layout-preset";
import type { Problem, Discussion, PublicSolution, User } from "@/lib/types";
import type { ProblemTabValue } from "./tabs";
import { PresetIde3Pane } from "./preset-ide-3pane";
import { PresetDoc2Pane } from "./preset-doc-2pane";
import { PresetTerminalBottom } from "./preset-terminal-bottom";
import { PresetFocusRead } from "./preset-focus-read";

export interface WorkspaceProps {
  preset: LayoutPreset;
  problem: Problem;
  activeTab: ProblemTabValue;
  discussions: Discussion[];
  solutions: PublicSolution[];
  userMap: Map<string, User>;
  user: Pick<User, "github_handle" | "is_pro">;
}

export function ProblemWorkspace(props: WorkspaceProps) {
  switch (props.preset) {
    case "ide-3pane":
      return <PresetIde3Pane {...props} />;
    case "doc-2pane":
      return <PresetDoc2Pane {...props} />;
    case "terminal-bottom":
      return <PresetTerminalBottom {...props} />;
    case "focus-read":
      return <PresetFocusRead {...props} />;
  }
}
