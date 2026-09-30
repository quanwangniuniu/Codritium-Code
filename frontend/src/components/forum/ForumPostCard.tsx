import Link from "next/link";
import { ArrowBigUp, Eye, MessageCircle, Pin } from "lucide-react";
import { compactCount } from "@/shared/format";
import { t } from "@/lib/i18n";
import {
  forumTimeAgo,
  FORUM_SECTION_LABEL_KEY,
  type ForumPost,
  type ForumViewer,
} from "@/lib/forum";
import { ForumAuthorName, ForumAvatar } from "@/components/forum/ForumAvatar";
import { ForumPostMenu } from "@/components/forum/ForumPostMenu";

interface ForumPostCardProps {
  post: ForumPost;
  viewer: ForumViewer | null;
  // Shown on "For You", where posts from every section are mixed.
  showSection?: boolean;
  onDeleted?: () => void;
}

// One row of the feed: author line, title, two-line preview, then
// votes · views · comments and the "…" menu — LeetCode's discuss card.
export function ForumPostCard({ post, viewer, showSection, onDeleted }: ForumPostCardProps) {
  return (
    <article className="flex gap-3 border-b border-divider py-5 last:border-b-0">
      <ForumAvatar author={post.author} />
      <div className="min-w-0 flex-1 space-y-1.5">
        <div className="flex flex-wrap items-center gap-x-1.5 gap-y-0.5 text-sm text-muted">
          <ForumAuthorName author={post.author} anonymous={post.is_anonymous && !!post.author} />
          <span aria-hidden>·</span>
          <time dateTime={post.created_at} suppressHydrationWarning>
            {forumTimeAgo(post.created_at)}
          </time>
          {showSection && (
            <>
              <span aria-hidden>·</span>
              <Link href={`/forums?section=${post.section}`} className="hover:text-ink">
                {t(FORUM_SECTION_LABEL_KEY[post.section])}
              </Link>
            </>
          )}
          {post.is_pinned && (
            <span className="ml-1 inline-flex items-center gap-1 text-xs text-accent">
              <Pin size={12} />
              {t("forum_pinned")}
            </span>
          )}
        </div>
        <h2 className="text-lg font-medium leading-snug">
          <Link href={`/forums/${post.id}`} className="text-ink hover:text-accent">
            {post.title}
          </Link>
        </h2>
        {post.excerpt && (
          <p className="line-clamp-2 text-sm leading-relaxed text-muted">{post.excerpt}</p>
        )}
        <div className="flex items-center gap-5 pt-1 text-sm text-muted">
          <span className="inline-flex items-center gap-1" title={t("forum_votes")}>
            <ArrowBigUp size={18} />
            {compactCount(post.score)}
          </span>
          <span className="inline-flex items-center gap-1" title={t("forum_views")}>
            <Eye size={16} />
            {compactCount(post.view_count)}
          </span>
          <Link
            href={`/forums/${post.id}#comments`}
            className="inline-flex items-center gap-1 hover:text-ink"
            title={t("forum_comments")}
          >
            <MessageCircle size={16} />
            {compactCount(post.comment_count)}
          </Link>
          {post.tags.length > 0 && (
            <span className="hidden min-w-0 items-center gap-2 truncate sm:flex">
              {post.tags.map((tag) => (
                <Link
                  key={tag}
                  href={`/forums?tag=${encodeURIComponent(tag)}`}
                  className="text-xs text-faint hover:text-accent"
                >
                  #{tag}
                </Link>
              ))}
            </span>
          )}
          <span className="ml-auto">
            <ForumPostMenu post={post} viewer={viewer} onDeleted={onDeleted} />
          </span>
        </div>
      </div>
    </article>
  );
}
