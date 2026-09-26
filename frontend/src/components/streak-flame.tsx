import { Flame } from "lucide-react";
import { cn } from "@/lib/utils";

interface StreakFlameProps {
  count: number;
  size?: "sm" | "md" | "lg";
  className?: string;
}

const SIZES = {
  sm: { wrapper: "text-xs gap-1", icon: 14 },
  md: { wrapper: "text-sm gap-1.5", icon: 16 },
  lg: { wrapper: "text-2xl gap-2 font-semibold", icon: 24 },
};

export function StreakFlame({ count, size = "md", className }: StreakFlameProps) {
  const cfg = SIZES[size];
  const tone =
    count >= 30
      ? "text-orange-500"
      : count >= 7
        ? "text-amber-500"
        : "text-faint";
  return (
    <span
      className={cn("inline-flex items-center", cfg.wrapper, tone, className)}
      aria-label={`Streak ${count} days`}
    >
      <Flame size={cfg.icon} fill={count > 0 ? "currentColor" : "none"} />
      <span>{count}</span>
    </span>
  );
}
