import Link from "next/link";
import {
  Bookmark,
  MessageCircle,
  ChevronRight,
  CircleCheck,
  FileText,
  History,
  ListChecks,
  MessageSquare,
  ThumbsUp,
  type LucideIcon,
} from "lucide-react";
import { SubmissionsList } from "@/features/submissions/components/SubmissionsList";
import { t, type LocaleKey } from "@/shared/i18n";
import type { Problem } from "@/features/problems/types";
import type { ProfileData } from "@/features/profile/types";
import type { Submission } from "@/features/submissions/types";
import type { ForumPost, MyForumComment } from "@/features/forum/api";
import { cn } from "@/shared/lib/cn";
import { DIFFICULTY_HOME_TEXT_CLASS, DIFFICULTY_LABEL_KEY } from "@/shared/labels";
import { formatRelativeTime } from "@/shared/format";

export type ProfileTab = "recent" | "solved" | "posts" | "comments" | "saved" | "all";

export const PROFILE_TABS: { id: ProfileTab; icon: LucideIcon; labelKey: LocaleKey }[] = [
  { id: "recent", icon: History, labelKey: "prof_tab_recent" },
  { id: "solved", icon: ListChecks, labelKey: "prof_tab_solved" },
  { id: "posts", icon: MessageSquare, labelKey: "prof_tab_posts" },
  { id: "comments", icon: MessageCircle, labelKey: "prof_tab_comments" },
  { id: "saved", icon: Bookmark, labelKey: "prof_tab_saved" },
  { id: "all", icon: FileText, labelKey: "prof_tab_all" },
];

interface ProfileTabsProps {
  tab: ProfileTab;
  profile: ProfileData;
  // Only loaded for the "all" tab.
  allSubmissions?: Submission[];
  problemMap?: Record<string, Problem>;
  // Only loaded for the "saved" tab.
  savedPosts?: ForumPost[];
  // Only loaded for the "comments" tab.
  myComments?: MyForumComment[];
}

export function ProfileTabs({ tab, profile, allSubmissions, problemMap, savedPosts, myComments }: ProfileTabsProps) {
  return (
    <div className="rounded-2xl border border-divider bg-surface p-3 sm:p-4">
      <div className="flex items-center gap-2 overflow-x-auto">
        <nav className="flex gap-1" aria-label={t("prof_tabs_label")}>
          {PROFILE_TABS.map((tb) => (
            <Link
              key={tb.id}
              href={tb.id === "recent" ? "/profile" : `/profile?tab=${tb.id}`}
              scroll={false}
              aria-current={tab === tb.id ? "page" : undefined}
              className={cn(
                "inline-flex items-center gap-2 whitespace-nowrap rounded-xl px-3.5 py-2.5 text-sm transition-colors",
                tab === tb.id ? "bg-surface-2 text-ink font-medium" : "text-muted hover:text-ink",
              )}
            >
              <tb.icon size={16} />
              {t(tb.labelKey)}
            </Link>
          ))}
        </nav>
        {tab !== "all" && (
          <Link
            href="/profile?tab=all"
            scroll={false}
            className="ml-auto hidden md:inline-flex items-center gap-0.5 whitespace-nowrap text-sm text-muted hover:text-ink"
          >
            {t("prof_view_all")}
            <ChevronRight size={16} />
          </Link>
        )}
      </div>

      <div className="mt-2">
        {tab === "recent" && <RecentList items={profile.recent_submissions} />}
        {tab === "solved" && <SolvedList items={profile.solved_problems} />}
        {tab === "posts" && <PostsList items={profile.recent_posts} />}
        {tab === "comments" && <CommentsList items={myComments ?? []} />}
        {tab === "saved" && <SavedList items={savedPosts ?? []} />}
        {tab === "all" && (
          <div className="p-1 sm:p-2">
            {allSubmissions && allSubmissions.length > 0 ? (
              <SubmissionsList initialSubmissions={allSubmissions} problemMap={problemMap ?? {}} />
            ) : (
              <Empty icon={FileText} textKey="prof_empty_submissions" />
            )}
          </div>
        )}
      </div>
    </div>
  );
}

function Row({ href, children }: { href: string; children: React.ReactNode }) {
  return (
    <li>
      <Link
        href={href}
        className="flex items-center gap-4 rounded-xl px-3.5 py-3 text-sm hover:bg-surface-2 transition-colors"
      >
        {children}
      </Link>
    </li>
  );
}

function RecentList({ items }: { items: ProfileData["recent_submissions"] }) {
  if (items.length === 0) return <Empty icon={History} textKey="prof_empty_submissions" />;
  return (
    <ul className="divide-y divide-divider/60">
      {items.map((s) => {
        const passed = s.status === "graded" && (s.final_score ?? 0) >= 60;
        return (
          <Row key={s.id} href={`/submissions/${s.id}`}>
            <span className="min-w-0 flex-1 truncate font-medium">{s.problem_title}</span>
            <span className={cn("hidden sm:inline text-xs", DIFFICULTY_HOME_TEXT_CLASS[s.difficulty])}>{t(DIFFICULTY_LABEL_KEY[s.difficulty])}</span>
            <ScorePill status={s.status} score={s.final_score} passed={passed} />
            <span className="shrink-0 whitespace-nowrap text-right text-xs text-faint sm:w-24">{formatRelativeTime(s.submitted_at)}</span>
          </Row>
        );
      })}
    </ul>
  );
}

function ScorePill({ status, score, passed }: { status: string; score: number | null; passed: boolean }) {
  if (status !== "graded" || score === null) {
    const key: LocaleKey = status === "failed" ? "prof_status_failed" : "prof_status_grading";
    return <span className="rounded-full bg-surface-2 px-2.5 py-0.5 text-xs text-muted">{t(key)}</span>;
  }
  return (
    <span
      className={cn(
        "rounded-full px-2.5 py-0.5 text-xs font-medium tabular-nums",
        passed ? "bg-home-green-soft text-home-green" : "bg-home-amber-soft text-home-amber",
      )}
    >
      {Math.round(score)}
    </span>
  );
}

function SolvedList({ items }: { items: ProfileData["solved_problems"] }) {
  if (items.length === 0) return <Empty icon={CircleCheck} textKey="prof_empty_solved" />;
  return (
    <ul className="divide-y divide-divider/60">
      {items.map((p) => (
        <Row key={p.slug} href={`/problems/${p.slug}`}>
          <CircleCheck size={16} className="shrink-0 text-home-green" />
          <span className="min-w-0 flex-1 truncate font-medium">{p.title}</span>
          <span className={cn("hidden sm:inline text-xs", DIFFICULTY_HOME_TEXT_CLASS[p.difficulty])}>{t(DIFFICULTY_LABEL_KEY[p.difficulty])}</span>
          <span className="text-xs text-muted tabular-nums">
            {t("prof_best_score", { params: { n: Math.round(p.best_score) } })}
          </span>
          <span className="shrink-0 whitespace-nowrap text-right text-xs text-faint sm:w-24">{formatRelativeTime(p.solved_at)}</span>
        </Row>
      ))}
    </ul>
  );
}

function PostsList({ items }: { items: ProfileData["recent_posts"] }) {
  if (items.length === 0) return <Empty icon={MessageSquare} textKey="prof_empty_posts" href="/forums/new" ctaKey="prof_empty_posts_cta" />;
  return (
    <ul className="divide-y divide-divider/60">
      {items.map((p) => (
        <Row key={p.id} href={`/forums/${p.id}`}>
          <span className="min-w-0 flex-1 truncate font-medium">{p.title}</span>
          <span className="hidden sm:inline-flex items-center gap-1 text-xs text-muted tabular-nums">
            <ThumbsUp size={13} /> {p.upvotes}
          </span>
          <span className="hidden sm:inline-flex items-center gap-1 text-xs text-muted tabular-nums">
            <MessageSquare size={13} /> {p.comment_count}
          </span>
          <span className="shrink-0 whitespace-nowrap text-right text-xs text-faint sm:w-24">{formatRelativeTime(p.created_at)}</span>
        </Row>
      ))}
    </ul>
  );
}

function CommentsList({ items }: { items: MyForumComment[] }) {
  if (items.length === 0) return <Empty icon={MessageCircle} textKey="prof_empty_comments" href="/forums" ctaKey="prof_empty_saved_cta" />;
  return (
    <ul className="divide-y divide-divider/60">
      {items.map((c) => (
        <Row key={c.id} href={`/forums/${c.post_id}#comment-${c.id}`}>
          <span className="min-w-0 flex-1">
            <span className="block truncate font-medium">{c.excerpt}</span>
            <span className="block truncate text-xs text-muted">
              {t("prof_comment_on_fmt", { params: { title: c.post_title } })}
              {c.is_anonymous && ` · ${t("prof_comment_anonymous")}`}
            </span>
          </span>
          <span className="hidden sm:inline-flex items-center gap-1 text-xs text-muted tabular-nums">
            <ThumbsUp size={13} /> {c.score}
          </span>
          <span className="shrink-0 whitespace-nowrap text-right text-xs text-faint sm:w-24">{formatRelativeTime(c.created_at)}</span>
        </Row>
      ))}
    </ul>
  );
}

function SavedList({ items }: { items: ForumPost[] }) {
  if (items.length === 0) return <Empty icon={Bookmark} textKey="prof_empty_saved" href="/forums" ctaKey="prof_empty_saved_cta" />;
  return (
    <ul className="divide-y divide-divider/60">
      {items.map((p) => (
        <Row key={p.id} href={`/forums/${p.id}`}>
          <span className="min-w-0 flex-1 truncate font-medium">{p.title}</span>
          <span className="hidden sm:inline-flex items-center gap-1 text-xs text-muted tabular-nums">
            <ThumbsUp size={13} /> {p.score}
          </span>
          <span className="hidden sm:inline-flex items-center gap-1 text-xs text-muted tabular-nums">
            <MessageSquare size={13} /> {p.comment_count}
          </span>
          <span className="shrink-0 whitespace-nowrap text-right text-xs text-faint sm:w-24">{formatRelativeTime(p.created_at)}</span>
        </Row>
      ))}
    </ul>
  );
}

function Empty({
  icon: Icon,
  textKey,
  href = "/problems",
  ctaKey = "prof_empty_cta",
}: {
  icon: LucideIcon;
  textKey: LocaleKey;
  href?: string;
  ctaKey?: LocaleKey;
}) {
  return (
    <div className="flex flex-col items-center justify-center text-center py-16 px-4">
      <span className="grid size-20 place-items-center rounded-3xl bg-surface-2 text-faint">
        <Icon size={34} strokeWidth={1.5} />
      </span>
      <p className="mt-5 text-muted">{t(textKey)}</p>
      <Link href={href} className="mt-3 text-sm font-medium text-accent hover:underline underline-offset-4">
        {t(ctaKey)}
      </Link>
    </div>
  );
}
