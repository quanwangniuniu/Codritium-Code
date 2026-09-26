import { CodeBlock } from "@/components/code-block";
import { SubmissionForm } from "@/components/submission-form";
import { ProblemTabs } from "./tabs";
import { ChatPlaceholder } from "./chat-placeholder";
import { EditorStatusBar } from "./status-bar";
import type { WorkspaceProps } from "./index";

export function PresetTerminalBottom({
  problem,
  activeTab,
  discussions,
  solutions,
  userMap,
  user,
}: WorkspaceProps) {
  return (
    <div className="flex flex-col h-[calc(100vh-3.5rem-1.5rem)] border border-divider rounded-md overflow-hidden bg-canvas">
      <div className="grid lg:grid-cols-2 gap-0 flex-1 min-h-0 border-b border-divider">
        <div className="border-r border-divider p-3 min-h-0 overflow-hidden flex flex-col">
          <ProblemTabs
            problem={problem}
            activeTab={activeTab}
            discussions={discussions}
            solutions={solutions}
            userMap={userMap}
          />
        </div>

        <div className="p-3 min-h-0 overflow-y-auto space-y-3">
          <div className="flex items-center justify-between">
            <h2 className="text-xs font-mono uppercase tracking-wider text-muted">
              ── starter ──
            </h2>
            <span className="text-[10px] text-faint font-mono">
              {Object.keys(problem.starter_files).length} file
              {Object.keys(problem.starter_files).length === 1 ? "" : "s"}
            </span>
          </div>
          {Object.entries(problem.starter_files).map(([fn, code]) => (
            <CodeBlock key={fn} filename={fn} code={code} />
          ))}

          <div className="pt-3 border-t border-divider space-y-2">
            <h2 className="text-xs font-mono uppercase tracking-wider text-muted">
              ── submission ──
            </h2>
            <SubmissionForm problem={problem} />
          </div>
        </div>
      </div>

      <div className="h-72 flex-shrink-0 bg-surface min-h-0">
        <ChatPlaceholder variant="terminal" problemId={problem.id} />
      </div>

      <EditorStatusBar user={user} problem={problem} position={{ line: 1, col: 1 }} />
    </div>
  );
}
