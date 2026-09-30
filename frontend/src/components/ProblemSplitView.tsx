"use client";

import { useCallback, useRef, useState, type ReactNode } from "react";
import { Splitter } from "@/components/Splitter";
import { useLocale } from "@/lib/i18n-client";
import { cn } from "@/lib/utils";

export interface ProblemTab {
  id: string;
  label: string;
  icon: ReactNode;
  content: ReactNode;
}

interface ProblemSplitViewProps {
  tabs: ProblemTab[];
  side: ReactNode;
}

const MIN_LEFT_PCT = 35;
const MAX_LEFT_PCT = 75;

// LeetCode-style problem page: a tabbed problem pane on the left and the
// start pane on the right, split by a draggable handle on wide screens and
// stacked on narrow ones. Tabs mount on first visit and then stay mounted,
// so switching back to Discussion doesn't refetch its comments.
export function ProblemSplitView({ tabs, side }: ProblemSplitViewProps) {
  useLocale();
  const containerRef = useRef<HTMLDivElement>(null);
  const [leftPct, setLeftPct] = useState(58);
  const [active, setActive] = useState(tabs[0]?.id ?? "");
  const [visited, setVisited] = useState<Set<string>>(() => new Set([tabs[0]?.id ?? ""]));

  const onResize = useCallback((dx: number) => {
    const width = containerRef.current?.clientWidth;
    if (!width) return;
    setLeftPct((pct) =>
      Math.min(MAX_LEFT_PCT, Math.max(MIN_LEFT_PCT, pct + (dx / width) * 100)),
    );
  }, []);

  function select(id: string) {
    setActive(id);
    setVisited((prev) => (prev.has(id) ? prev : new Set(prev).add(id)));
  }

  return (
    <div
      ref={containerRef}
      className="flex flex-col gap-3 p-3 lg:h-[calc(100dvh-3.5rem-1px)] lg:flex-row lg:gap-0"
      style={{ "--left-pct": `${leftPct}%` } as React.CSSProperties}
    >
      <section className="flex min-h-0 min-w-0 flex-col overflow-hidden rounded-lg border border-divider bg-surface lg:w-[var(--left-pct)]">
        <div
          role="tablist"
          className="flex shrink-0 gap-1 overflow-x-auto border-b border-divider bg-surface-2/60 px-2 py-1.5"
        >
          {tabs.map((tab) => (
            <button
              key={tab.id}
              type="button"
              role="tab"
              id={`tab-${tab.id}`}
              aria-selected={active === tab.id}
              aria-controls={`panel-${tab.id}`}
              onClick={() => select(tab.id)}
              className={cn(
                "flex shrink-0 items-center gap-1.5 rounded-md px-3 py-1.5 text-sm transition-colors",
                active === tab.id
                  ? "bg-surface font-medium text-ink shadow-sm"
                  : "text-muted hover:text-ink",
              )}
            >
              {tab.icon}
              {tab.label}
            </button>
          ))}
        </div>
        <div className="min-h-0 flex-1 overflow-y-auto">
          {tabs.map((tab) =>
            visited.has(tab.id) ? (
              <div
                key={tab.id}
                role="tabpanel"
                id={`panel-${tab.id}`}
                aria-labelledby={`tab-${tab.id}`}
                hidden={active !== tab.id}
                className="px-5 py-5 sm:px-6"
              >
                {tab.content}
              </div>
            ) : null,
          )}
        </div>
      </section>

      <div className="hidden lg:flex">
        <Splitter onResize={onResize} />
      </div>

      <aside className="min-h-0 min-w-0 overflow-y-auto rounded-lg border border-divider bg-surface lg:flex-1">
        {side}
      </aside>
    </div>
  );
}
