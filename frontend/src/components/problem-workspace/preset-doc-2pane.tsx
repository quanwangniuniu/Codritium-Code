import { CodeBlock } from "@/components/code-block";
import { SubmissionForm } from "@/components/submission-form";
import { ProblemTabs } from "./tabs";
import type { WorkspaceProps } from "./index";

export function PresetDoc2Pane({
  problem,
  activeTab,
  discussions,
  solutions,
  userMap,
}: WorkspaceProps) {
  return (
    <div className="grid lg:grid-cols-2 gap-6 items-start">
      <div className="space-y-3 lg:sticky lg:top-20 lg:max-h-[calc(100vh-6rem)]">
        <ProblemTabs
          problem={problem}
          activeTab={activeTab}
          discussions={discussions}
          solutions={solutions}
          userMap={userMap}
        />
      </div>

      <div className="space-y-4">
        <section className="space-y-2">
          <div className="flex items-center justify-between">
            <h2 className="text-sm font-semibold text-ink">Starter</h2>
            <span className="text-xs text-faint font-mono">
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

        <section className="space-y-3 pt-4 border-t border-divider">
          <h2 className="text-sm font-semibold text-ink">Your submission</h2>
          <p className="text-xs text-muted">
            Edit the starter, paste the prompt history that produced your fix, and submit.
          </p>
          <SubmissionForm problem={problem} />
        </section>
      </div>
    </div>
  );
}
