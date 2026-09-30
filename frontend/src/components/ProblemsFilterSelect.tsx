"use client";

import { useRouter } from "next/navigation";
import { ChevronDown } from "lucide-react";
import { useLocale } from "@/shared/i18n/client";
import { cn } from "@/lib/utils";

interface ProblemsFilterSelectProps {
  label: string;
  // Each option carries its own precomputed href so the server page stays the
  // single owner of URL/filter semantics; this component only navigates.
  options: { value: string; label: string; href: string }[];
  active: string;
}

export function ProblemsFilterSelect({ label, options, active }: ProblemsFilterSelectProps) {
  useLocale();
  const router = useRouter();
  const isFiltered = active !== "all";

  return (
    <label
      className={cn(
        "relative inline-flex items-center rounded-md border text-xs transition-colors",
        isFiltered
          ? "border-accent bg-accent-soft text-accent"
          : "border-divider bg-surface text-muted hover:border-divider-strong hover:text-ink",
      )}
    >
      <span className="sr-only">{label}</span>
      <select
        value={active}
        onChange={(e) => {
          const next = options.find((o) => o.value === e.target.value);
          if (next) router.push(next.href);
        }}
        className="cursor-pointer appearance-none bg-transparent py-1.5 pl-3 pr-7 outline-none"
      >
        {options.map((o) => (
          <option key={o.value} value={o.value}>
            {o.value === "all" ? label : o.label}
          </option>
        ))}
      </select>
      <ChevronDown size={14} className="pointer-events-none absolute right-2" />
    </label>
  );
}
