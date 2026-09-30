import Link from "next/link";
import { CircleCheck, CircleDashed, Lock, Search } from "lucide-react";
import { listProblems, listMyAttemptedProblems, listUserSubmissions } from "@/lib/store";
import { ProblemsPagination } from "@/components/ProblemsPagination";
import { ProblemsFilterSelect } from "@/components/ProblemsFilterSelect";
import { t, type LocaleKey } from "@/lib/i18n";
import { cn } from "@/lib/utils";
import type { Category, Difficulty } from "@/lib/types";

type CategoryFilter = "all" | Category;
type StatusFilter = "all" | "solved" | "attempted" | "todo";
type ProblemStatus = Exclude<StatusFilter, "all">;
type FilterKey = "category" | "difficulty" | "status" | "company" | "q";

const CATEGORY_FILTERS: { value: CategoryFilter; labelKey: LocaleKey }[] = [
  { value: "all", labelKey: "problems_filter_all" },
  { value: "debugging", labelKey: "profile_category_debugging" },
  { value: "feature_build", labelKey: "profile_category_feature_build" },
  { value: "refactoring", labelKey: "profile_category_refactoring" },
  { value: "security", labelKey: "profile_category_security" },
  { value: "company_premium", labelKey: "profile_category_company_premium" },
];

const DIFFICULTY_FILTERS: { value: "all" | Difficulty; labelKey: LocaleKey }[] = [
  { value: "all", labelKey: "problems_filter_all" },
  { value: "easy", labelKey: "problems_difficulty_easy" },
  { value: "medium", labelKey: "problems_difficulty_medium" },
  { value: "hard", labelKey: "problems_difficulty_hard" },
];

const STATUS_FILTERS: { value: StatusFilter; labelKey: LocaleKey }[] = [
  { value: "all", labelKey: "problems_filter_all" },
  { value: "todo", labelKey: "problems_status_todo" },
  { value: "attempted", labelKey: "problems_status_attempted" },
  { value: "solved", labelKey: "problems_status_solved" },
];

const CATEGORY_LABEL_KEY: Record<Category, LocaleKey> = {
  debugging: "profile_category_debugging",
  feature_build: "profile_category_feature_build",
  refactoring: "profile_category_refactoring",
  security: "profile_category_security",
  company_premium: "profile_category_company_premium",
};

const DIFFICULTY_LABEL_KEY: Record<Difficulty, LocaleKey> = {
  easy: "problems_difficulty_easy",
  medium: "problems_difficulty_medium",
  hard: "problems_difficulty_hard",
};

const DIFFICULTY_TEXT: Record<Difficulty, string> = {
  easy: "text-success",
  medium: "text-warning",
  hard: "text-danger",
};

const PAGE_SIZE = 50;
const VALID_CATEGORY_VALUES = new Set<string>(CATEGORY_FILTERS.map((f) => f.value));
const VALID_DIFFICULTY_VALUES = new Set<string>(DIFFICULTY_FILTERS.map((f) => f.value));
const VALID_STATUS_VALUES = new Set<string>(STATUS_FILTERS.map((f) => f.value));

interface ProblemsPageProps {
  searchParams: Promise<{
    category?: string;
    difficulty?: string;
    status?: string;
    company?: string;
    q?: string;
    page?: string;
  }>;
}

export default async function ProblemsPage({ searchParams }: ProblemsPageProps) {
  const sp = await searchParams;
  const fCategory = (VALID_CATEGORY_VALUES.has(sp.category ?? "") ? sp.category : "all") as CategoryFilter;
  const fDifficulty = (
    VALID_DIFFICULTY_VALUES.has(sp.difficulty ?? "") ? sp.difficulty : "all"
  ) as "all" | Difficulty;
  const fStatus = (VALID_STATUS_VALUES.has(sp.status ?? "") ? sp.status : "all") as StatusFilter;
  const fCompany = sp.company ?? "all";
  const fQuery = (sp.q ?? "").trim();
  const requestedPage = Math.max(1, parseInt(sp.page ?? "1", 10) || 1);

  const [problems, attempted, submissions] = await Promise.all([
    listProblems(),
    listMyAttemptedProblems(),
    listUserSubmissions(""),
  ]);

  // Solved = at least one graded submission; attempted = a session was
  // started but nothing has been graded yet.
  const solvedSlugs = new Set(
    submissions.filter((s) => s.status === "completed").map((s) => s.problem_id),
  );
  const attemptedSlugs = new Set(attempted.map((a) => a.slug));
  function statusOf(slug: string): ProblemStatus {
    if (solvedSlugs.has(slug)) return "solved";
    if (attemptedSlugs.has(slug)) return "attempted";
    return "todo";
  }

  const needle = fQuery.toLowerCase();
  const filtered = problems.filter((p) => {
    if (fCategory !== "all" && p.category !== fCategory) return false;
    if (fDifficulty !== "all" && p.difficulty !== fDifficulty) return false;
    if (fStatus !== "all" && statusOf(p.id) !== fStatus) return false;
    if (fCompany !== "all" && !p.company_slugs.includes(fCompany)) return false;
    if (needle && !p.title.toLowerCase().includes(needle)) return false;
    return true;
  });

  const companies = Array.from(new Set(problems.flatMap((p) => p.company_slugs))).sort();
  const solvedCount = problems.filter((p) => solvedSlugs.has(p.id)).length;
  const hasFilter =
    fCategory !== "all" || fDifficulty !== "all" || fStatus !== "all" || fCompany !== "all" || !!fQuery;

  const totalPages = Math.max(1, Math.ceil(filtered.length / PAGE_SIZE));
  const safePage = Math.min(requestedPage, totalPages);
  const pageItems = filtered.slice((safePage - 1) * PAGE_SIZE, safePage * PAGE_SIZE);

  // Active filters, carried into every filter link, the search form and the
  // pagination component so changing one filter preserves the others.
  const baseParams: Record<string, string> = {};
  if (fCategory !== "all") baseParams.category = fCategory;
  if (fDifficulty !== "all") baseParams.difficulty = fDifficulty;
  if (fStatus !== "all") baseParams.status = fStatus;
  if (fCompany !== "all") baseParams.company = fCompany;
  if (fQuery) baseParams.q = fQuery;

  function filterHref(key: FilterKey, value: string) {
    const params = new URLSearchParams(baseParams);
    params.delete(key);
    if (value !== "all") params.set(key, value);
    // Any filter change rewinds to page 1 — the previous page index might
    // now be out of range after the result set shrinks or shifts.
    const qs = params.toString();
    return qs ? `/problems?${qs}` : "/problems";
  }

  function selectOptions(filters: { value: string; labelKey: LocaleKey }[], key: FilterKey) {
    return filters.map((o) => ({ value: o.value, label: t(o.labelKey), href: filterHref(key, o.value) }));
  }

  return (
    <div className="mx-auto max-w-[1200px] space-y-5 px-4 py-8 sm:px-6 sm:py-10">
      <header className="flex flex-wrap items-end justify-between gap-4">
        <h1 className="text-3xl font-semibold tracking-tight">{t("problems_title")}</h1>
        <form action="/problems" className="relative w-full sm:w-72">
          {Object.entries(baseParams)
            .filter(([k]) => k !== "q")
            .map(([k, v]) => (
              <input key={k} type="hidden" name={k} value={v} />
            ))}
          <Search
            size={14}
            className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-faint"
          />
          <input
            type="search"
            name="q"
            defaultValue={fQuery}
            placeholder={t("problems_search_placeholder")}
            aria-label={t("problems_search_placeholder")}
            className="w-full rounded-md border border-divider bg-surface py-2 pl-9 pr-3 text-sm text-ink outline-none placeholder:text-faint focus:border-accent"
          />
        </form>
      </header>

      <nav className="-mx-4 overflow-x-auto px-4 sm:mx-0 sm:px-0" aria-label={t("problems_filter_category")}>
        <div className="flex w-max gap-2">
          {CATEGORY_FILTERS.map((o) => (
            <Link
              key={o.value}
              href={filterHref("category", o.value)}
              aria-current={fCategory === o.value ? "page" : undefined}
              className={cn(
                "whitespace-nowrap rounded-full px-4 py-1.5 text-sm transition-colors",
                fCategory === o.value
                  ? "bg-ink text-canvas"
                  : "bg-surface-2 text-muted hover:text-ink",
              )}
            >
              {t(o.labelKey)}
            </Link>
          ))}
        </div>
      </nav>

      <div className="flex flex-wrap items-center gap-2">
        <ProblemsFilterSelect
          label={t("problems_filter_difficulty")}
          options={selectOptions(DIFFICULTY_FILTERS, "difficulty")}
          active={fDifficulty}
        />
        <ProblemsFilterSelect
          label={t("problems_filter_status")}
          options={selectOptions(STATUS_FILTERS, "status")}
          active={fStatus}
        />
        {companies.length > 0 && (
          <ProblemsFilterSelect
            label={t("problems_filter_company")}
            options={[
              { value: "all", label: t("problems_filter_all"), href: filterHref("company", "all") },
              ...companies.map((c) => ({ value: c, label: c, href: filterHref("company", c) })),
            ]}
            active={fCompany}
          />
        )}
        {hasFilter && (
          <Link href="/problems" className="text-xs text-accent hover:underline">
            {t("problems_clear_filters")}
          </Link>
        )}
        <span className="ml-auto flex items-center gap-1.5 text-xs text-muted">
          <CircleCheck size={14} className="text-success" />
          {t("problems_solved_count_fmt", { params: { count: solvedCount } })}
        </span>
      </div>

      {filtered.length === 0 ? (
        <div className="space-y-3 rounded-md border border-divider py-16 text-center">
          <p className="text-muted">{t("problems_no_match")}</p>
          {hasFilter && (
            <Link href="/problems" className="text-sm text-accent underline">
              {t("problems_clear_filters")}
            </Link>
          )}
        </div>
      ) : (
        <>
          <div className="overflow-hidden rounded-md border border-divider">
            <div
             
              className="grid grid-cols-[2.5rem_1fr_5rem] items-center gap-3 border-b border-divider bg-surface px-3 py-2.5 text-xs font-medium text-muted sm:grid-cols-[3.5rem_1fr_9rem_6rem] sm:px-4"
            >
              <span>{t("problems_filter_status")}</span>
              <span>{t("problems_col_title")}</span>
              <span className="hidden sm:block">
                {t("problems_filter_category")}
              </span>
              <span>{t("problems_filter_difficulty")}</span>
            </div>
            <ul>
              {pageItems.map((p) => {
                const status = statusOf(p.id);
                return (
                  <li key={p.id} className="odd:bg-canvas even:bg-surface">
                    <Link
                      href={`/problems/${p.id}`}
                      className="grid grid-cols-[2.5rem_1fr_5rem] items-center gap-3 px-3 py-2.5 text-sm transition-colors hover:bg-surface-2 sm:grid-cols-[3.5rem_1fr_9rem_6rem] sm:px-4"
                    >
                      <StatusIcon status={status} />
                      <span className="flex min-w-0 items-center gap-2">
                        <span className="truncate text-ink">{p.title}</span>
                        {p.is_pro_only && (
                          <Lock
                            size={13}
                            className="shrink-0 text-warning"
                            aria-label={t("problems_pro_lock_tooltip")}
                          >
                            <title>{t("problems_pro_lock_tooltip")}</title>
                          </Lock>
                        )}
                      </span>
                      <span className="hidden truncate text-muted sm:block">
                        {t(CATEGORY_LABEL_KEY[p.category])}
                      </span>
                      <span className={cn("font-medium", DIFFICULTY_TEXT[p.difficulty])}>
                        {t(DIFFICULTY_LABEL_KEY[p.difficulty])}
                      </span>
                    </Link>
                  </li>
                );
              })}
            </ul>
          </div>
          {totalPages > 1 && (
            <ProblemsPagination
              currentPage={safePage}
              totalPages={totalPages}
              baseParams={baseParams}
            />
          )}
        </>
      )}
    </div>
  );
}

function StatusIcon({ status }: { status: ProblemStatus }) {
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
