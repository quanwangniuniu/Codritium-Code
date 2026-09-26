"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { t } from "@/lib/i18n";
import { useLocale } from "@/lib/i18n-client";
import { cn } from "@/lib/utils";

interface ProblemsPaginationProps {
  currentPage: number;
  // Total is used only to clamp jumps; it is intentionally never rendered, so
  // the catalog size and the last page stay hidden.
  totalPages: number;
  baseParams: Record<string, string>;
}

export function ProblemsPagination({
  currentPage,
  totalPages,
  baseParams,
}: ProblemsPaginationProps) {
  useLocale();
  const router = useRouter();
  const [value, setValue] = useState(String(currentPage));

  // Resync the input when navigation lands on a new page.
  useEffect(() => {
    setValue(String(currentPage));
  }, [currentPage]);

  function hrefFor(page: number) {
    const params = new URLSearchParams(baseParams);
    if (page > 1) params.set("page", String(page));
    const qs = params.toString();
    return qs ? `/problems?${qs}` : "/problems";
  }

  function go(page: number) {
    const clamped = Math.min(Math.max(1, page), totalPages);
    router.push(hrefFor(clamped));
  }

  function commit() {
    const n = parseInt(value, 10);
    if (Number.isFinite(n) && n >= 1) go(n);
    else setValue(String(currentPage));
  }

  const atStart = currentPage <= 1;
  const atEnd = currentPage >= totalPages;
  const btn =
    "min-w-[64px] rounded border px-3 py-1 text-center text-xs transition-colors";
  const enabled = "border-divider text-muted hover:border-divider-strong hover:text-ink";
  const off = "cursor-not-allowed border-divider text-faint";

  return (
    <nav
      className="flex items-center justify-center gap-2 border-t border-divider pt-4"
      aria-label="Pagination"
    >
      {atStart ? (
        <span aria-disabled className={cn(btn, off)}>
          {t("problems_page_prev")}
        </span>
      ) : (
        <button type="button" onClick={() => go(currentPage - 1)} className={cn(btn, enabled)}>
          {t("problems_page_prev")}
        </button>
      )}

      <form
        onSubmit={(e) => {
          e.preventDefault();
          commit();
        }}
        className="flex items-center gap-1.5"
      >
        <label htmlFor="page-jump" className="text-xs text-faint">
          {t("problems_page_word")}
        </label>
        <input
          id="page-jump"
          type="text"
          inputMode="numeric"
          value={value}
          onChange={(e) => setValue(e.target.value.replace(/[^0-9]/g, ""))}
          aria-label={t("problems_page_jump_aria")}
          className="w-12 rounded border border-divider bg-surface px-2 py-1 text-center text-xs text-ink outline-none focus:border-accent"
        />
      </form>

      {atEnd ? (
        <span aria-disabled className={cn(btn, off)}>
          {t("problems_page_next")}
        </span>
      ) : (
        <button type="button" onClick={() => go(currentPage + 1)} className={cn(btn, enabled)}>
          {t("problems_page_next")}
        </button>
      )}
    </nav>
  );
}
