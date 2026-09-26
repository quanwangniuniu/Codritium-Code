import { cn } from "@/lib/utils";

interface ScoreBarProps {
  label: string;
  score: number | null;
  weight: number;
  reasoning?: string;
}

export function ScoreBar({ label, score, weight, reasoning }: ScoreBarProps) {
  const pct = score !== null ? (score / 5) * 100 : 0;
  const tone =
    score === null
      ? "bg-surface-2"
      : score >= 4
        ? "bg-success"
        : score >= 3
          ? "bg-warning"
          : "bg-danger";

  return (
    <div className="space-y-1.5">
      <div className="flex items-baseline justify-between gap-3">
        <div className="flex items-baseline gap-2">
          <span className="text-sm font-medium text-ink">{label}</span>
          <span className="text-xs text-faint">weight {(weight * 100).toFixed(0)}%</span>
        </div>
        <span className={cn("text-sm tabular-nums", score === null ? "text-faint" : "text-ink")}>
          {score === null ? "—" : `${score.toFixed(2)} / 5`}
        </span>
      </div>
      <div className="h-2 rounded-full bg-surface-2 overflow-hidden">
        <div
          className={cn("h-full rounded-full transition-all", tone)}
          style={{ width: `${pct}%` }}
        />
      </div>
      {reasoning && (
        <p className="text-xs text-muted leading-relaxed">{reasoning}</p>
      )}
    </div>
  );
}
