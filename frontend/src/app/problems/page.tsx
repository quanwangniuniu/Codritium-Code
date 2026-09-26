import Link from "next/link";
import { ArrowRight, Lock } from "lucide-react";
import { listProblems, listMyAttemptedProblems } from "@/lib/store";
import { ProblemsPagination } from "@/components/ProblemsPagination";
import { t, type LocaleKey } from "@/lib/i18n";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";
import type { Category, Difficulty, Problem } from "@/lib/types";

type CategoryFilter = "all" | Category | "done";

const CATEGORY_FILTERS: { value: CategoryFilter; labelKey: LocaleKey }[] = [
  { value: "all", labelKey: "problems_filter_all" },
  { value: "done", labelKey: "problems_filter_done" },
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

const PAGE_SIZE = 8;
const VALID_CATEGORY_VALUES = new Set<CategoryFilter>(CATEGORY_FILTERS.map((f) => f.value));

interface ProblemsPageProps {
  searchParams: Promise<{
    category?: string;
    difficulty?: string;
    company?: string;
    page?: string;
  }>;
}

export default async function ProblemsPage({ searchParams }: ProblemsPageProps) {
  const sp = await searchParams;
  const rawCategory = (sp.category ?? "all") as CategoryFilter;
  const fCategory: CategoryFilter = VALID_CATEGORY_VALUES.has(rawCategory) ? rawCategory : "all";
  const fDifficulty = (sp.difficulty ?? "all") as "all" | Difficulty;
  const fCompany = sp.company ?? "all";
  const requestedPage = Math.max(1, parseInt(sp.page ?? "1", 10) || 1);

  // Done filter sources order from the user's candidate_sessions history;
  // every other filter goes through the static problem catalog.
  const [problems, attempted] = await Promise.all([
    listProblems(),
    fCategory === "done" ? listMyAttemptedProblems() : Promise.resolve([]),
  ]);

  const problemByID = new Map(problems.map((p) => [p.id, p]));

  let candidatePool: Problem[];
  if (fCategory === "done") {
    // Preserve the "most recently worked on first" ordering from backend.
    candidatePool = attempted
      .map((a) => problemByID.get(a.slug))
      .filter((p): p is Problem => Boolean(p));
  } else {
    candidatePool = problems;
  }

  const filtered = candidatePool.filter((p) => {
    if (fCategory !== "all" && fCategory !== "done" && p.category !== fCategory) return false;
    if (fDifficulty !== "all" && p.difficulty !== fDifficulty) return false;
    if (fCompany !== "all" && !p.company_slugs.includes(fCompany)) return false;
    return true;
  });

  const companies = Array.from(new Set(problems.flatMap((p) => p.company_slugs))).sort();
  const hasFilter = fCategory !== "all" || fDifficulty !== "all" || fCompany !== "all";

  const totalPages = Math.max(1, Math.ceil(filtered.length / PAGE_SIZE));
  const safePage = Math.min(requestedPage, totalPages);
  const pageItems = filtered.slice((safePage - 1) * PAGE_SIZE, safePage * PAGE_SIZE);

  function chipHref(key: "category" | "difficulty" | "company", value: string) {
    const params = new URLSearchParams();
    if (key !== "category" && fCategory !== "all") params.set("category", fCategory);
    if (key !== "difficulty" && fDifficulty !== "all") params.set("difficulty", fDifficulty);
    if (key !== "company" && fCompany !== "all") params.set("company", fCompany);
    if (value !== "all") params.set(key, value);
    // Any filter change rewinds to page 1 — the previous page index might
    // now be out of range after the result set shrinks or shifts.
    const qs = params.toString();
    return qs ? `/problems?${qs}` : "/problems";
  }

  // Active filters carried into the pagination component so jumps preserve them.
  const baseParams: Record<string, string> = {};
  if (fCategory !== "all") baseParams.category = fCategory;
  if (fDifficulty !== "all") baseParams.difficulty = fDifficulty;
  if (fCompany !== "all") baseParams.company = fCompany;

  return (
    <div className="mx-auto max-w-[1600px] px-6 py-10 space-y-6">
      <header>
        <h1 className="text-3xl tracking-tight font-semibold">{t("problems_title")}</h1>
      </header>

      <div className="space-y-3 rounded-md border border-divider bg-surface p-4">
        <FilterChipRow
          label={t("problems_filter_category")}
          options={CATEGORY_FILTERS.map((o) => ({ value: o.value, label: t(o.labelKey) }))}
          active={fCategory}
          hrefFn={(v) => chipHref("category", v)}
        />
        <FilterChipRow
          label={t("problems_filter_difficulty")}
          options={DIFFICULTY_FILTERS.map((o) => ({ value: o.value, label: t(o.labelKey) }))}
          active={fDifficulty}
          hrefFn={(v) => chipHref("difficulty", v)}
        />
        {companies.length > 0 && (
          <FilterChipRow
            label={t("problems_filter_company")}
            options={[
              { value: "all", label: t("problems_filter_all") },
              ...companies.map((c) => ({ value: c, label: c })),
            ]}
            active={fCompany}
            hrefFn={(v) => chipHref("company", v)}
          />
        )}
      </div>

      {filtered.length === 0 ? (
        <div className="space-y-3 py-16 text-center">
          <p className="text-muted">{t("problems_no_match")}</p>
          {hasFilter && (
            <Link href="/problems" className="text-sm text-accent underline">
              {t("problems_clear_filters")}
            </Link>
          )}
        </div>
      ) : (
        <>
          <div className="grid gap-4 sm:grid-cols-2">
            {pageItems.map((p) => (
              <Link key={p.id} href={`/problems/${p.id}`} className="group relative block">
                {p.is_pro_only && (
                  <div
                    className="absolute right-3 top-3 z-10 text-faint"
                    title={t("problems_pro_lock_tooltip")}
                    aria-label={t("problems_pro_lock_tooltip")}
                  >
                    <Lock size={14} />
                  </div>
                )}
                <Card className="flex h-[195px] flex-col overflow-hidden transition-colors group-hover:border-divider-strong">
                  <CardHeader className="pb-2">
                    <div className="mb-2 flex flex-wrap items-center gap-2">
                      <Badge tone="info">{t(CATEGORY_LABEL_KEY[p.category])}</Badge>
                      <Badge
                        tone={
                          p.difficulty === "easy"
                            ? "success"
                            : p.difficulty === "medium"
                              ? "warning"
                              : "danger"
                        }
                      >
                        {t(DIFFICULTY_LABEL_KEY[p.difficulty])}
                      </Badge>
                      {p.is_pro_only && <Badge tone="pro">{t("pro_badge")}</Badge>}
                    </div>
                    <CardTitle className="line-clamp-2 flex items-start gap-2">
                      <span className="flex-1">{p.title}</span>
                      <ArrowRight
                        size={16}
                        className="mt-1 shrink-0 opacity-0 transition-opacity group-hover:opacity-100"
                      />
                    </CardTitle>
                    <CardDescription className="mt-2 line-clamp-2 min-h-[2.5rem]">
                      {p.description_md
                        .split("\n")
                        .find((l) => l.trim() && !l.startsWith("#")) ?? ""}
                    </CardDescription>
                  </CardHeader>
                  <CardContent className="mt-auto">
                    <div className="flex h-6 flex-wrap items-start gap-1.5 overflow-hidden">
                      {p.company_slugs.slice(0, 3).map((slug) => (
                        <Badge key={slug} tone="neutral">
                          {slug}
                        </Badge>
                      ))}
                      {p.company_slugs.length > 3 && (
                        <Badge tone="neutral">+{p.company_slugs.length - 3}</Badge>
                      )}
                    </div>
                  </CardContent>
                </Card>
              </Link>
            ))}
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

interface FilterChipRowProps {
  label: string;
  options: { value: string; label: string }[];
  active: string;
  hrefFn: (v: string) => string;
}

function FilterChipRow({ label, options, active, hrefFn }: FilterChipRowProps) {
  return (
    <div className="flex flex-wrap items-center gap-2">
      <span className="min-w-[72px] text-xs uppercase tracking-wider text-faint">
        {label}
      </span>
      {options.map((o) => (
        <Link
          key={o.value}
          href={hrefFn(o.value)}
          aria-pressed={active === o.value}
          className={cn(
            "rounded border px-2.5 py-1 text-xs transition-colors",
            active === o.value
              ? "border-accent bg-accent-soft text-accent"
              : "border-divider text-muted hover:border-divider-strong hover:text-ink",
          )}
        >
          {o.label}
        </Link>
      ))}
    </div>
  );
}
