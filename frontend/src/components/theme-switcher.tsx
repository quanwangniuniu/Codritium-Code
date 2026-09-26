import { Palette } from "lucide-react";
import { THEMES, THEME_LABELS, type Theme } from "@/lib/theme";
import { cn } from "@/lib/utils";

interface ThemeSwitcherProps {
  current: Theme;
}

export function ThemeSwitcher({ current }: ThemeSwitcherProps) {
  return (
    <form
      action="/api/theme"
      method="post"
      className="hidden md:inline-flex items-center gap-1 rounded-md border border-divider bg-surface p-0.5"
    >
      <span className="px-1.5 text-faint" aria-label="Theme">
        <Palette size={12} />
      </span>
      {THEMES.map((t) => (
        <button
          key={t}
          type="submit"
          name="theme"
          value={t}
          aria-pressed={current === t}
          title={THEME_LABELS[t].tagline}
          className={cn(
            "px-2 py-0.5 text-xs rounded transition-colors",
            current === t
              ? "bg-accent text-accent-fg"
              : "text-muted hover:text-ink hover:bg-surface-2",
          )}
        >
          {THEME_LABELS[t].label}
        </button>
      ))}
    </form>
  );
}
