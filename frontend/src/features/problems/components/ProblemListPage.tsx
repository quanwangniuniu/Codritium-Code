import Link from "next/link";
import {
  Bug,
  Building2,
  CircleCheck,
  Hammer,
  LayoutGrid,
  Search,
  ShieldCheck,
  Shuffle,
  type LucideIcon,
} from "lucide-react";
import { getProblemFacets, searchProblems } from "@/features/problems/server";
import type { Category } from "@/features/problems/types";
import { ProblemInfiniteList } from "@/features/problems/components/ProblemInfiniteList";
import { ProblemTagRow } from "@/features/problems/components/ProblemTagRow";
import { ProblemsFilterSelect } from "@/features/problems/components/ProblemsFilterSelect";
import {
  hasFilters,
  parseFilters,
  problemsHref,
  USER_STATUSES,
  type FilterKey,
  type ProblemFilters,
  type UserStatus,
} from "@/features/problems/lib/search";
import { t, type LocaleKey } from "@/shared/i18n";
import { cn } from "@/shared/lib/cn";
import { CATEGORIES, CATEGORY_LABEL_KEY, DIFFICULTIES, DIFFICULTY_LABEL_KEY } from "@/shared/labels";

const CATEGORY_ICON: Record<Category, { icon: LucideIcon; className: string }> = {
  debugging: { icon: Bug, className: "text-warning" },
  feature_build: { icon: Hammer, className: "text-accent" },
  refactoring: { icon: Shuffle, className: "text-success" },
  security: { icon: ShieldCheck, className: "text-danger" },
  company_premium: { icon: Building2, className: "text-muted" },
};

const STATUS_LABEL_KEY: Record<UserStatus, LocaleKey> = {
  todo: "problems_status_todo",
  attempted: "problems_status_attempted",
  solved: "problems_status_solved",
};

interface ProblemsPageProps {
  searchParams: Promise<Partial<Record<FilterKey, string>>>;
}

export async function ProblemListPage({ searchParams }: ProblemsPageProps) {
  const filters = parseFilters(await searchParams);
  const [facets, firstPage] = await Promise.all([getProblemFacets(), searchProblems(filters)]);

  // Changing one filter keeps the others; picking the active value again
  // (or "all") clears it.
  function hrefWith(patch: ProblemFilters) {
    return problemsHref({ ...filters, ...patch });
  }

  const categoryCount = new Map(facets.categories.map((c) => [c.value, c.count]));
  const categoryPills = [
    { value: undefined, label: t("problems_filter_all_topics"), icon: LayoutGrid, iconClass: "" },
    ...CATEGORIES.filter((c) => categoryCount.get(c)).map((c) => ({
      value: c,
      label: t(CATEGORY_LABEL_KEY[c]),
      icon: CATEGORY_ICON[c].icon,
      iconClass: CATEGORY_ICON[c].className,
    })),
  ];

  const filtersKey = problemsHref(filters);

  return (
    <div className="mx-auto max-w-[1200px] space-y-5 px-4 py-8 sm:px-6 sm:py-10">
      <h1 className="text-3xl font-semibold tracking-tight">{t("problems_title")}</h1>

      {facets.tags.length > 0 && (
        <ProblemTagRow
          active={filters.tag}
          tags={facets.tags.map((tag) => ({
            ...tag,
            href: hrefWith({ tag: filters.tag === tag.value ? undefined : tag.value }),
          }))}
        />
      )}

      <nav
        className="-mx-4 overflow-x-auto border-b border-divider px-4 pb-5 sm:mx-0 sm:px-0"
        aria-label={t("problems_filter_category")}
      >
        <div className="flex w-max gap-3">
          {categoryPills.map((pill) => {
            const active = filters.category === pill.value;
            const Icon = pill.icon;
            return (
              <Link
                key={pill.value ?? "all"}
                href={hrefWith({ category: pill.value })}
                aria-current={active ? "page" : undefined}
                className={cn(
                  "flex items-center gap-2 whitespace-nowrap rounded-full px-4 py-2 text-sm transition-colors",
                  active ? "bg-ink font-medium text-canvas" : "bg-surface-2 text-muted hover:text-ink",
                )}
              >
                <Icon size={16} className={active ? undefined : pill.iconClass} />
                {pill.label}
              </Link>
            );
          })}
        </div>
      </nav>

      <div className="flex flex-wrap items-center gap-2">
        <form action="/problems" className="relative w-full sm:w-64">
          {(["category", "difficulty", "status", "tag"] as const).map(
            (k) => filters[k] && <input key={k} type="hidden" name={k} value={filters[k]} />,
          )}
          <Search
            size={14}
            className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-faint"
          />
          <input
            type="search"
            name="q"
            defaultValue={filters.q ?? ""}
            placeholder={t("problems_search_placeholder")}
            aria-label={t("problems_search_placeholder")}
            className="w-full rounded-full bg-surface-2 py-2 pl-9 pr-3 text-sm text-ink outline-none placeholder:text-faint focus:ring-1 focus:ring-accent"
          />
        </form>
        <ProblemsFilterSelect
          label={t("problems_filter_difficulty")}
          active={filters.difficulty ?? "all"}
          options={[
            { value: "all", label: t("problems_filter_all"), href: hrefWith({ difficulty: undefined }) },
            ...DIFFICULTIES.map((d) => ({
              value: d,
              label: t(DIFFICULTY_LABEL_KEY[d]),
              href: hrefWith({ difficulty: d }),
            })),
          ]}
        />
        <ProblemsFilterSelect
          label={t("problems_filter_status")}
          active={filters.status ?? "all"}
          options={[
            { value: "all", label: t("problems_filter_all"), href: hrefWith({ status: undefined }) },
            ...USER_STATUSES.map((s) => ({
              value: s,
              label: t(STATUS_LABEL_KEY[s]),
              href: hrefWith({ status: s }),
            })),
          ]}
        />
        {hasFilters(filters) && (
          <Link href="/problems" className="text-xs text-accent hover:underline">
            {t("problems_clear_filters")}
          </Link>
        )}
        <span className="ml-auto flex items-center gap-1.5 text-sm text-muted">
          <CircleCheck size={15} className={facets.solved > 0 ? "text-success" : "text-faint"} />
          {t("problems_solved_count_fmt", { params: { count: facets.solved, total: facets.total } })}
        </span>
      </div>

      {firstPage.items.length === 0 ? (
        <div className="space-y-3 rounded-md border border-divider py-16 text-center">
          <p className="text-muted">{t("problems_no_match")}</p>
          {hasFilters(filters) && (
            <Link href="/problems" className="text-sm text-accent underline">
              {t("problems_clear_filters")}
            </Link>
          )}
        </div>
      ) : (
        <ProblemInfiniteList key={filtersKey} initial={firstPage} filters={filters} />
      )}
    </div>
  );
}
