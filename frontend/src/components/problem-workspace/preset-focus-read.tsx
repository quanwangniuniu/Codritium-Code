import { CodeBlock } from "@/components/code-block";
import { SubmissionForm } from "@/components/submission-form";
import { ProblemTabs } from "./tabs";
import { ChatDrawerToggle } from "./chat-drawer-toggle";
import { EditorStatusBar } from "./status-bar";
import type { WorkspaceProps } from "./index";

export function PresetFocusRead({
  problem,
  activeTab,
  discussions,
  solutions,
  userMap,
  user,
}: WorkspaceProps) {
  return (
    <div className="relative flex flex-col h-[calc(100vh-3.5rem-1.5rem)] border border-divider rounded-md overflow-hidden bg-canvas">
      <div className="flex-1 min-h-0 overflow-y-auto">
        <div className="mx-auto max-w-3xl px-6 py-6 space-y-5">
          <ProblemTabs
            problem={problem}
            activeTab={activeTab}
            discussions={discussions}
            solutions={solutions}
            userMap={userMap}
          />

          <section className="space-y-2 pt-2">
            <div className="flex items-center justify-between">
              <h2 className="text-xs font-mono uppercase tracking-wider text-muted">
                ── starter ──
              </h2>
              <span className="text-[10px] text-faint font-mono">
                {Object.keys(problem.starter_files).length} file
                {Object.keys(problem.starter_files).length === 1 ? "" : "s"}
              </span>
            </div>
            <div className="space-y-2">
              {Object.entries(problem.starter_files).map(([fn, code]) => (
                <CodeBlock key={fn} filename={fn} code={code} />
              ))}
            </div>
          </section>

          <section className="space-y-3 pt-3 border-t border-divider">
            <h2 className="text-xs font-mono uppercase tracking-wider text-muted">
              ── submission ──
            </h2>
            <SubmissionForm problem={problem} />
          </section>
        </div>
      </div>

      <ChatDrawerToggle problemId={problem.id} />

      <EditorStatusBar user={user} problem={problem} position={{ line: 1, col: 1 }} />
    </div>
  );
}
