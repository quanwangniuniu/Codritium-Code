import {
  Files,
  BookOpen,
  MessageSquare,
  Star,
  Settings,
  Sparkles,
} from "lucide-react";
import { CodeBlock } from "@/components/code-block";
import { SubmissionForm } from "@/components/submission-form";
import { ProblemTabs } from "./tabs";
import { ChatPlaceholder } from "./chat-placeholder";
import { EditorStatusBar } from "./status-bar";
import type { WorkspaceProps } from "./index";

const ACTIVITY_ITEMS = [
  { icon: Files, label: "Files" },
  { icon: BookOpen, label: "Description" },
  { icon: MessageSquare, label: "Discussion" },
  { icon: Star, label: "Solutions" },
  { icon: Sparkles, label: "Agent" },
];

export function PresetIde3Pane({
  problem,
  activeTab,
  discussions,
  solutions,
  userMap,
  user,
}: WorkspaceProps) {
  return (
    <div className="flex flex-col h-[calc(100vh-3.5rem-1.5rem)] border border-divider rounded-md overflow-hidden bg-canvas">
      <div className="flex flex-1 min-h-0">
        <aside className="w-12 flex-shrink-0 bg-surface-2 border-r border-divider flex flex-col items-center py-2 gap-1">
          {ACTIVITY_ITEMS.map((it) => (
            <button
              key={it.label}
              className="p-2 text-muted hover:text-ink hover:bg-surface rounded transition-colors"
              title={it.label}
              aria-label={it.label}
            >
              <it.icon size={16} />
            </button>
          ))}
          <div className="flex-1" />
          <button
            className="p-2 text-faint hover:text-ink"
            title="Settings"
            aria-label="Settings"
          >
            <Settings size={16} />
          </button>
        </aside>

        <aside className="w-72 flex-shrink-0 bg-surface border-r border-divider p-3 min-h-0 overflow-hidden">
          <ProblemTabs
            problem={problem}
            activeTab={activeTab}
            discussions={discussions}
            solutions={solutions}
            userMap={userMap}
          />
        </aside>

        <main className="flex-1 min-w-0 flex flex-col bg-canvas overflow-hidden">
          <div className="flex items-end gap-0 px-2 pt-2 border-b border-divider bg-surface-2 flex-shrink-0 overflow-x-auto">
            {Object.keys(problem.starter_files).map((fn, i) => (
              <div
                key={fn}
                className={
                  i === 0
                    ? "px-3 py-1.5 text-xs font-mono bg-canvas border border-divider border-b-0 rounded-t text-ink"
                    : "px-3 py-1.5 text-xs font-mono text-muted hover:text-ink"
                }
              >
                {fn}
              </div>
            ))}
          </div>

          <div className="flex-1 min-h-0 overflow-y-auto p-3 space-y-3">
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
        </main>

        <aside className="w-80 flex-shrink-0 border-l border-divider bg-surface min-h-0">
          <ChatPlaceholder variant="panel" problemId={problem.id} />
        </aside>
      </div>

      <EditorStatusBar
        user={user}
        problem={problem}
        position={{ line: 1, col: 1 }}
      />
    </div>
  );
}
