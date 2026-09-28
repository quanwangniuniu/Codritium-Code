import { AlertTriangle, CircleHelp, ShieldCheck } from "lucide-react";
import { cn } from "@/lib/utils";
import { t, type LocaleKey } from "@/lib/i18n";

const LABEL_KEYS: Record<string, LocaleKey> = {
  hands_off: "antipattern_label_hands_off",
  feature_marathon: "antipattern_label_feature_marathon",
  ai_showcase: "antipattern_label_ai_showcase",
  not_thinking: "antipattern_label_not_thinking",
};

interface AntiPatternChipProps {
  name: keyof typeof LABEL_KEYS | string;
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
  const key = LABEL_KEYS[name];
  const label = key ? t(key) : name;

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
