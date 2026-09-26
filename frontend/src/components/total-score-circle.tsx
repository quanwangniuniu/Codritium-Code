import { cn } from "@/lib/utils";

interface TotalScoreCircleProps {
  total: number;
  size?: number;
}

export function TotalScoreCircle({ total, size = 132 }: TotalScoreCircleProps) {
  const radius = (size - 12) / 2;
  const circumference = 2 * Math.PI * radius;
  const pct = Math.max(0, Math.min(100, total));
  const offset = circumference * (1 - pct / 100);

  const tone =
    total >= 80
      ? "stroke-success"
      : total >= 60
        ? "stroke-warning"
        : "stroke-danger";
  const textTone =
    total >= 80 ? "text-success" : total >= 60 ? "text-warning" : "text-danger";

  return (
    <div className="relative inline-flex items-center justify-center" style={{ width: size, height: size }}>
      <svg width={size} height={size} className="-rotate-90">
        <circle
          cx={size / 2}
          cy={size / 2}
          r={radius}
          strokeWidth={10}
          className="stroke-surface-2"
          fill="none"
        />
        <circle
          cx={size / 2}
          cy={size / 2}
          r={radius}
          strokeWidth={10}
          className={cn("transition-all", tone)}
          fill="none"
          strokeLinecap="round"
          strokeDasharray={circumference}
          strokeDashoffset={offset}
        />
      </svg>
      <div className="absolute inset-0 flex flex-col items-center justify-center">
        <span className={cn("text-3xl font-semibold tabular-nums", textTone)}>
          {total.toFixed(0)}
        </span>
        <span className="text-xs text-faint">out of 100</span>
      </div>
    </div>
  );
}
