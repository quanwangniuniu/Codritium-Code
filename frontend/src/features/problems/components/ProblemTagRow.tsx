"use client";

import { useState } from "react";
import Link from "next/link";
import { ChevronsDown, ChevronsUp } from "lucide-react";
import { t } from "@/shared/i18n";
import { useLocale } from "@/shared/i18n/client";
import { cn } from "@/shared/lib/cn";

interface ProblemTagRowProps {
  // Each tag carries its own precomputed href so the server page stays the
  // single owner of URL/filter semantics; the active tag's href clears it.
  tags: { value: string; count: number; href: string }[];
  active?: string;
}

// Topic tags with their problem counts: one clipped line until expanded.
export function ProblemTagRow({ tags, active }: ProblemTagRowProps) {
  useLocale();
  const [expanded, setExpanded] = useState(false);

  return (
    <nav aria-label={t("problems_tags_aria")} className="relative">
      <div
        className={cn(
          "flex gap-x-5 gap-y-2 text-sm",
          expanded ? "flex-wrap pr-24" : "h-7 flex-nowrap overflow-hidden",
        )}
      >
        {tags.map((tag) => {
          const isActive = tag.value === active;
          return (
            <Link
              key={tag.value}
              href={tag.href}
              aria-current={isActive ? "true" : undefined}
              className={cn(
                "group flex shrink-0 items-center gap-1.5 whitespace-nowrap transition-colors",
                isActive ? "font-medium text-accent" : "text-ink hover:text-accent",
              )}
            >
              {tag.value}
              <span
                className={cn(
                  "rounded-full px-1.5 py-0.5 text-xs",
                  isActive ? "bg-accent-soft text-accent" : "bg-surface-2 text-muted",
                )}
              >
                {tag.count}
              </span>
            </Link>
          );
        })}
      </div>
      <button
        type="button"
        onClick={() => setExpanded((v) => !v)}
        aria-expanded={expanded}
        className={cn(
          "absolute right-0 flex h-7 items-center gap-1 bg-canvas pl-3 text-sm text-muted hover:text-ink",
          expanded ? "bottom-0" : "top-0 shadow-[-16px_0_12px_var(--color-canvas)]",
        )}
      >
        {t(expanded ? "problems_tags_collapse" : "problems_tags_expand")}
        {expanded ? <ChevronsUp size={14} /> : <ChevronsDown size={14} />}
      </button>
    </nav>
  );
}
