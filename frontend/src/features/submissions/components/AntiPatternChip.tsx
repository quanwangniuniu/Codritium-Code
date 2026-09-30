import { AlertTriangle, CircleHelp, ShieldCheck } from "lucide-react";
import { cn } from "@/shared/lib/cn";
import { t } from "@/shared/i18n";
import { antiPatternLabel } from "@/shared/labels";

interface AntiPatternChipProps {
  name: string;
  evaluated: boolean;
  triggered: boolean;
  evidence: string;
}

export function AntiPatternChip({
  name,
  evaluated,
  triggered,
  evidence,
}: AntiPatternChipProps) {
  const label = antiPatternLabel(name);

  return (
    <div
      className={cn(
        "rounded-lg border p-3 text-sm",
        !evaluated
          ? "border-divider bg-surface"
          : triggered
            ? "border-danger/30 bg-danger-soft"
            : "border-success/25 bg-success-soft",
      )}
    >
      <div className="flex items-center gap-2 font-medium">
        {!evaluated ? (
          <CircleHelp size={16} className="text-muted" />
        ) : triggered ? (
          <AlertTriangle size={16} className="text-danger" />
        ) : (
          <ShieldCheck size={16} className="text-success" />
        )}

        <span
          className={
            !evaluated
              ? "text-muted"
              : triggered
                ? "text-danger"
                : "text-success"
          }
        >
          {label}
        </span>

        <span
          className={cn(
            "ml-auto text-xs",
            !evaluated
              ? "text-muted"
              : triggered
                ? "text-danger/80"
                : "text-success/80",
          )}
        >
          {!evaluated
            ? t("antipattern_not_evaluated")
            : triggered
              ? t("antipattern_flagged")
              : t("antipattern_clear")}
        </span>
      </div>

      {(!evaluated || triggered) && evidence && (
        <p
          className={cn(
            "mt-1.5 text-xs",
            evaluated ? "text-danger/90" : "text-muted",
          )}
        >
          {evidence}
        </p>
      )}
    </div>
  );
}
