import type { CSSProperties } from "react";
import type { LucideIcon } from "lucide-react";
import { cn } from "@/shared/lib/cn";

export type HomeAccent = "blue" | "green" | "amber" | "teal" | "rose";

// Text colour utility per accent, for section headings that match their badge.
export const ACCENT_TEXT: Record<HomeAccent, string> = {
  blue: "text-home-blue",
  green: "text-home-green",
  amber: "text-home-amber",
  teal: "text-home-teal",
  rose: "text-home-rose",
};

interface SectionBadgeProps {
  icon: LucideIcon;
  accent: HomeAccent;
  className?: string;
}

// Rounded accent tile that heads each home section (the .home-badge style
// lives in globals.css; --badge picks the accent token).
export function SectionBadge({ icon: Icon, accent, className }: SectionBadgeProps) {
  return (
    <span
      aria-hidden
      className={cn("home-badge", className)}
      style={{ "--badge": `var(--home-${accent})` } as CSSProperties}
    >
      <Icon size={24} strokeWidth={2} />
    </span>
  );
}
