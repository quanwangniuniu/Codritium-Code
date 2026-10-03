"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import Link from "next/link";
import { CircleCheck, CircleDashed, Lock } from "lucide-react";
import { problemsApi, type ProblemSearchItem, type ProblemSearchPage } from "@/features/problems/api";
import { searchQuery, type ProblemFilters } from "@/features/problems/lib/search";
import { t } from "@/shared/i18n";
import { useLocale } from "@/shared/i18n/client";
import { cn } from "@/shared/lib/cn";
import { categoryLabel, difficultyLabel, DIFFICULTY_TEXT_CLASS, type Difficulty } from "@/shared/labels";

const ROW_GRID =
  "grid grid-cols-[1.5rem_1fr_4.5rem] items-center gap-3 px-3 sm:grid-cols-[1.5rem_1fr_9rem_4.5rem] sm:px-4";

interface ProblemInfiniteListProps {
  // The server-rendered first page. The parent remounts this component (via
  // `key`) whenever the filters change, so state never mixes two result sets.
  initial: ProblemSearchPage;
  filters: ProblemFilters;
}

// The problem rows. Fetches the next page whenever the sentinel under the
// last row scrolls near the viewport, until the backend reports no more.
export function ProblemInfiniteList({ initial, filters }: ProblemInfiniteListProps) {
  useLocale();
  const [items, setItems] = useState(initial.items);
  const [nextOffset, setNextOffset] = useState(initial.next_offset);
  const [loading, setLoading] = useState(false);
  const [failed, setFailed] = useState(false);
  const sentinel = useRef<HTMLDivElement>(null);
  // Guards against a second fetch for the same offset while one is in flight;
  // state alone is too slow because the observer can fire twice in a frame.
  const inFlight = useRef(false);

  const loadMore = useCallback(async () => {
    if (nextOffset === null || inFlight.current) return;
    inFlight.current = true;
    setLoading(true);
    setFailed(false);
    try {
      const page = await problemsApi.search(searchQuery(filters, nextOffset));
      // The catalog can shift between requests; never render a slug twice.
      setItems((prev) => {
        const seen = new Set(prev.map((p) => p.slug));
        return [...prev, ...page.items.filter((p) => !seen.has(p.slug))];
      });
      setNextOffset(page.next_offset);
    } catch {
      setFailed(true);
    } finally {
      inFlight.current = false;
      setLoading(false);
    }
  }, [filters, nextOffset]);

  useEffect(() => {
    const el = sentinel.current;
    // After a failure, wait for the explicit retry instead of hammering.
    if (!el || nextOffset === null || failed) return;
    const observer = new IntersectionObserver(
      (entries) => {
        if (entries.some((e) => e.isIntersecting)) void loadMore();
      },
      { rootMargin: "600px 0px" },
    );
    observer.observe(el);
    return () => observer.disconnect();
  }, [loadMore, nextOffset, failed]);

  return (
    <div>
      <ul>
        {items.map((p) => (
          <ProblemRow key={p.slug} problem={p} />
        ))}
      </ul>

      {nextOffset !== null && !failed && (
        <div ref={sentinel} aria-busy={loading} aria-label={t("problems_loading_more")}>
          {Array.from({ length: 3 }, (_, i) => (
            <div key={i} className={cn(ROW_GRID, "h-11 animate-pulse rounded-lg")}>
              <span />
              <span className="h-3 w-2/5 rounded bg-surface-2" />
              <span className="hidden h-3 w-16 rounded bg-surface-2 sm:block" />
              <span className="h-3 w-10 rounded bg-surface-2" />
            </div>
          ))}
        </div>
      )}

      {failed && (
        <p role="alert" className="py-6 text-center text-sm text-muted">
          {t("problems_load_failed")}{" "}
          <button type="button" onClick={() => void loadMore()} className="text-accent underline">
            {t("problems_load_retry")}
          </button>
        </p>
      )}

      {nextOffset === null && items.length > initial.items.length && (
        <p className="py-6 text-center text-xs text-faint">{t("problems_list_end")}</p>
      )}
    </div>
  );
}

function ProblemRow({ problem: p }: { problem: ProblemSearchItem }) {
  return (
    <li className="rounded-lg odd:bg-surface">
      <Link
        href={`/problems/${p.slug}`}
        className={cn(ROW_GRID, "h-11 rounded-lg text-sm transition-colors hover:bg-surface-2")}
      >
        <StatusIcon status={p.user_status} />
        <span className="flex min-w-0 items-center gap-2">
          <span className="truncate text-ink">{p.title}</span>
          {p.requires_pro && (
            <Lock size={13} className="shrink-0 text-warning" aria-label={t("problems_pro_lock_tooltip")}>
              <title>{t("problems_pro_lock_tooltip")}</title>
            </Lock>
          )}
        </span>
        <span className="hidden truncate text-muted sm:block">{categoryLabel(p.category)}</span>
        <span className={cn("text-right font-medium", DIFFICULTY_TEXT_CLASS[p.difficulty as Difficulty])}>
          {difficultyLabel(p.difficulty)}
        </span>
      </Link>
    </li>
  );
}

function StatusIcon({ status }: { status: ProblemSearchItem["user_status"] }) {
  if (status === "solved") {
    return (
      <CircleCheck size={16} className="text-success" aria-label={t("problems_status_solved")}>
        <title>{t("problems_status_solved")}</title>
      </CircleCheck>
    );
  }
  if (status === "attempted") {
    return (
      <CircleDashed size={16} className="text-warning" aria-label={t("problems_status_attempted")}>
        <title>{t("problems_status_attempted")}</title>
      </CircleDashed>
    );
  }
  return <span aria-label={t("problems_status_todo")} />;
}
