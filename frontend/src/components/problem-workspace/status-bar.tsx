import type { Problem, User } from "@/lib/types";

interface EditorStatusBarProps {
  user: Pick<User, "github_handle" | "is_pro">;
  problem: Pick<Problem, "id" | "category" | "difficulty">;
  position?: { line: number; col: number };
}

export function EditorStatusBar({
  user,
  problem,
  position = { line: 1, col: 1 },
}: EditorStatusBarProps) {
  return (
    <div className="flex items-center justify-between gap-4 px-3 h-7 text-[11px] font-mono bg-surface-2 border-t border-divider text-muted flex-shrink-0">
      <div className="flex items-center gap-2 min-w-0">
        <span className="text-success">❯</span>
        <span className="text-ink truncate">{user.github_handle}</span>
        <span>@</span>
        <span className="text-ink truncate">{problem.id}</span>
        {user.is_pro && <span className="text-accent">pro</span>}
      </div>
      <div className="flex items-center gap-3 flex-shrink-0">
        <span>{problem.category}</span>
        <span>{problem.difficulty}</span>
        <span>
          Ln {position.line}, Col {position.col}
        </span>
        <span>UTF-8</span>
      </div>
    </div>
  );
}
