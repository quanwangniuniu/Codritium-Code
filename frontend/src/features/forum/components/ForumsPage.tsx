import Link from "next/link";
import { Code2, Eye, Flame, Hash, Pin, Search, SquarePen, Sparkles, TrendingUp, X } from "lucide-react";
import { compactCount } from "@/shared/format";
import { currentUser } from "@/features/auth/server";
import { t } from "@/shared/i18n";
import { cn } from "@/shared/lib/cn";
import {
  FORUM_SECTIONS,
  FORUM_SECTION_LABEL_KEY,
  type ForumFeedQuery,
  type ForumPost,
  type ForumSection,
  type ForumSort,
  type ForumTagCount,
  type ForumTrendingItem,
  type ForumViewer,
} from "@/features/forum/api";
import { getForumFeed, getForumTags, getPinnedForumPosts, getTrendingForumPosts } from "@/features/forum/server";
import { getProblem } from "@/features/problems/server";
import { ForumFeed } from "@/features/forum/components/ForumFeed";
import { ForumAuthorName, ForumAvatar } from "@/features/forum/components/ForumAvatar";

interface ForumsPageProps {
  searchParams: Promise<{ section?: string; sort?: string; q?: string; tag?: string; problem?: string }>;
}

export async function ForumsPage({ searchParams }: ForumsPageProps) {
  const sp = await searchParams;
  const section = FORUM_SECTIONS.includes(sp.section as ForumSection)
    ? (sp.section as ForumSection)
    : undefined;
  const sort: ForumSort = sp.sort === "votes" || sp.sort === "newest" ? sp.sort : "hot";
  const q = (sp.q ?? "").trim().slice(0, 100);
  const tag = (sp.tag ?? "").trim().replace(/^#/, "").toLowerCase();
  const problem = (sp.problem ?? "").trim();
  const query: ForumFeedQuery = {
    section,
    sort,
    q: q || undefined,
    tag: tag || undefined,
    problem: problem || undefined,
  };

  // Pinned posts headline the landing view only, like LeetCode's banners.
  const showPinned = !section && !q && !tag && !problem;
  const [user, feed, pinned, trending, popularTags, tagInfo, problemInfo] = await Promise.all([
    currentUser(),
    getForumFeed(query),
    showPinned ? getPinnedForumPosts() : Promise.resolve([] as ForumPost[]),
    getTrendingForumPosts(),
    getForumTags(),
    // Tags matching the prefix include the exact one (if any post has it).
    tag ? getForumTags(tag, 50) : Promise.resolve([] as ForumTagCount[]),
    problem ? getProblem(problem) : Promise.resolve(null),
  ]);
  const tagCount = tagInfo.find((x) => x.tag === tag)?.count ?? 0;
  const viewer: ForumViewer | null = user ? { id: user.id, isAdmin: user.role === "admin" } : null;

  function href(next: Partial<Record<"section" | "sort" | "q" | "tag" | "problem", string | undefined>>) {
    const merged = {
      section,
      sort: sort === "hot" ? undefined : sort,
      q: q || undefined,
      tag: tag || undefined,
      problem: problem || undefined,
      ...next,
    };
    const params = new URLSearchParams();
    for (const [k, v] of Object.entries(merged)) if (v) params.set(k, v);
    const qs = params.toString();
    return qs ? `/forums?${qs}` : "/forums";
  }

  const newPostPath = problem ? `/forums/new?problem=${encodeURIComponent(problem)}` : "/forums/new";
  const createHref = user ? newPostPath : `/login?next=${encodeURIComponent(newPostPath)}`;
  const searchForm = (
    <form action="/forums" className="relative">
      {section && <input type="hidden" name="section" value={section} />}
      <Search size={15} className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-faint" />
      <input
        type="search"
        name="q"
        defaultValue={q}
        placeholder={t("forum_search_placeholder")}
        aria-label={t("forum_search_placeholder")}
        className="w-full rounded-full border border-divider bg-surface-2 py-2 pl-9 pr-4 text-sm text-ink outline-none placeholder:text-faint focus:border-accent"
      />
    </form>
  );

  return (
    <div className="mx-auto grid max-w-[1300px] gap-8 px-4 py-6 sm:px-6 lg:grid-cols-[minmax(0,1fr)_300px] lg:py-8">
      <div className="min-w-0 space-y-5">
        <div className="lg:hidden">{searchForm}</div>

        {tag && (
          <header className="flex items-center gap-3 rounded-xl border border-divider bg-surface p-4">
            <span className="grid size-10 place-items-center rounded-lg bg-accent-soft text-accent">
              <Hash size={20} />
            </span>
            <div>
              <h1 className="text-lg font-semibold">{tag}</h1>
              <p className="text-sm text-muted">{t("forum_tag_posts_fmt", { params: { n: tagCount } })}</p>
            </div>
          </header>
        )}
        {problem && (
          <header className="flex flex-wrap items-center gap-3 rounded-xl border border-divider bg-surface p-4">
            <span className="grid size-10 place-items-center rounded-lg bg-accent-soft text-accent">
              <Code2 size={20} />
            </span>
            <div className="min-w-0">
              <p className="text-xs text-muted">{t("forum_problem_posts_label")}</p>
              <h1 className="truncate text-lg font-semibold">{problemInfo?.title ?? problem}</h1>
            </div>
            <Link href={`/problems/${encodeURIComponent(problem)}`} className="ml-auto text-sm text-accent hover:underline">
              {t("forum_open_problem")}
            </Link>
          </header>
        )}

        {pinned.length > 0 && (
          <section aria-label={t("forum_pinned")} className="-mx-4 overflow-x-auto px-4 sm:mx-0 sm:px-0">
            <div className="flex w-max gap-3 sm:grid sm:w-auto sm:grid-cols-2 xl:grid-cols-3">
              {pinned.map((p) => (
                <Link
                  key={p.id}
                  href={`/forums/${p.id}`}
                  className="flex w-72 flex-col gap-2 rounded-xl border border-divider bg-gradient-to-br from-accent-soft to-surface p-4 transition-colors hover:border-accent sm:w-auto"
                >
                  <span className="inline-flex items-center gap-1 text-xs font-medium text-accent">
                    <Pin size={12} />
                    {t("forum_pinned")}
                  </span>
                  <span className="line-clamp-2 font-semibold leading-snug text-ink">{p.title}</span>
                  <span className="line-clamp-2 text-xs leading-relaxed text-muted">{p.excerpt}</span>
                  <span className="mt-auto flex items-center gap-2 pt-1 text-xs text-muted">
                    <ForumAvatar author={p.author} size={18} />
                    <ForumAuthorName author={p.author} anonymous={false} />
                  </span>
                </Link>
              ))}
            </div>
          </section>
        )}

        <div className="flex items-center gap-3">
          <nav aria-label={t("forum_sections")} className="-mx-4 min-w-0 flex-1 overflow-x-auto px-4 sm:mx-0 sm:px-0">
            <div className="flex w-max gap-1">
              <SectionTab href={href({ section: undefined })} active={!section}>
                <Flame size={15} />
                {t("forum_for_you")}
              </SectionTab>
              {FORUM_SECTIONS.map((s) => (
                <SectionTab key={s} href={href({ section: s })} active={section === s}>
                  {t(FORUM_SECTION_LABEL_KEY[s])}
                </SectionTab>
              ))}
            </div>
          </nav>
          <Link
            href={createHref}
            className="inline-flex shrink-0 items-center gap-1.5 rounded-md bg-success px-3.5 py-2 text-sm font-medium text-white hover:opacity-90"
          >
            <SquarePen size={15} />
            {t("forum_create")}
          </Link>
        </div>

        <div className="flex flex-wrap items-center gap-x-5 gap-y-2 text-sm">
          <SortLink href={href({ sort: sort === "votes" ? undefined : "votes" })} active={sort === "votes"}>
            <TrendingUp size={15} />
            {t("forum_sort_votes")}
          </SortLink>
          <SortLink href={href({ sort: sort === "newest" ? undefined : "newest" })} active={sort === "newest"}>
            <Sparkles size={15} />
            {t("forum_sort_newest")}
          </SortLink>
          {q && (
            <FilterChip href={href({ q: undefined })}>
              {t("forum_results_for_fmt", { params: { q } })}
            </FilterChip>
          )}
          {tag && <FilterChip href={href({ tag: undefined })}>#{tag}</FilterChip>}
          {problem && <FilterChip href={href({ problem: undefined })}>{problemInfo?.title ?? problem}</FilterChip>}
        </div>

        <ForumFeed
          key={JSON.stringify(query)}
          initialPosts={feed.posts}
          initialHasMore={feed.has_more}
          asOf={feed.as_of}
          query={query}
          viewer={viewer}
        />
      </div>

      {/* Below an infinitely scrolling feed the sidebar would be unreachable on
          narrow screens, so it only shows on desktop (search moves above the feed). */}
      <aside className="hidden space-y-4 lg:block">
        {searchForm}
        <TrendingList items={trending} />
        <PopularTags tags={popularTags} active={tag} />
      </aside>
    </div>
  );
}

function SectionTab({ href, active, children }: { href: string; active: boolean; children: React.ReactNode }) {
  return (
    <Link
      href={href}
      aria-current={active ? "page" : undefined}
      className={cn(
        "inline-flex items-center gap-1.5 whitespace-nowrap rounded-md px-3 py-1.5 text-[15px] transition-colors",
        active ? "bg-surface-2 font-semibold text-ink" : "text-muted hover:text-ink",
      )}
    >
      {children}
    </Link>
  );
}

function SortLink({ href, active, children }: { href: string; active: boolean; children: React.ReactNode }) {
  return (
    <Link
      href={href}
      aria-pressed={active}
      className={cn(
        "inline-flex items-center gap-1.5 transition-colors",
        active ? "font-medium text-ink" : "text-muted hover:text-ink",
      )}
    >
      {children}
    </Link>
  );
}

function FilterChip({ href, children }: { href: string; children: React.ReactNode }) {
  return (
    <Link
      href={href}
      className="inline-flex items-center gap-1 rounded-full bg-accent-soft px-2.5 py-0.5 text-xs text-accent hover:opacity-80"
      aria-label={t("forum_clear_filter")}
    >
      {children}
      <X size={12} />
    </Link>
  );
}

function PopularTags({ tags, active }: { tags: ForumTagCount[]; active: string }) {
  if (tags.length === 0) return null;
  return (
    <section className="rounded-xl border border-divider bg-surface p-4">
      <h2 className="mb-3 flex items-center gap-2 text-lg font-semibold">
        <Hash size={18} />
        {t("forum_popular_tags")}
      </h2>
      <div className="flex flex-wrap gap-1.5">
        {tags.map((x) => (
          <Link
            key={x.tag}
            href={x.tag === active ? "/forums" : `/forums?tag=${encodeURIComponent(x.tag)}`}
            aria-current={x.tag === active ? "page" : undefined}
            className={cn(
              "inline-flex items-center gap-1 rounded-full px-2.5 py-1 text-xs transition-colors",
              x.tag === active ? "bg-accent text-accent-fg" : "bg-surface-2 text-muted hover:text-ink",
            )}
          >
            #{x.tag}
            <span className="tabular-nums opacity-70">{x.count}</span>
          </Link>
        ))}
      </div>
    </section>
  );
}

const RANK_COLORS = ["text-danger", "text-warning", "text-warning"];

function TrendingList({ items }: { items: ForumTrendingItem[] }) {
  return (
    <section className="rounded-xl border border-divider bg-surface p-4">
      <h2 className="mb-3 flex items-center gap-2 text-lg font-semibold">
        <TrendingUp size={18} />
        {t("forum_trending")}
      </h2>
      {items.length === 0 ? (
        <p className="text-sm text-muted">{t("forum_trending_empty")}</p>
      ) : (
        <ol className="space-y-3">
          {items.map((it, i) => (
            <li key={it.id} className="flex gap-3">
              <span className={cn("w-4 shrink-0 text-sm font-medium tabular-nums", RANK_COLORS[i] ?? "text-muted")}>
                {i + 1}
              </span>
              <div className="min-w-0">
                <Link href={`/forums/${it.id}`} className="line-clamp-2 text-sm leading-snug text-ink hover:text-accent">
                  {it.title}
                </Link>
                <div className="mt-0.5 flex items-center gap-2 text-xs text-faint">
                  <span>#{it.tag || t(FORUM_SECTION_LABEL_KEY[it.section])}</span>
                  <span aria-hidden>·</span>
                  <span className="inline-flex items-center gap-1">
                    <Eye size={12} />
                    {compactCount(it.view_count)}
                  </span>
                </div>
              </div>
            </li>
          ))}
        </ol>
      )}
    </section>
  );
}
