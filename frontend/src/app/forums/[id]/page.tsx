import Link from "next/link";
import { notFound } from "next/navigation";
import { compactCount } from "@/shared/format";
import { ArrowLeft, Code2, Eye, MessageCircle, Pin } from "lucide-react";
import { currentUser } from "@/lib/auth";
import { t } from "@/shared/i18n";
import {
  forumTimeAgo,
  FORUM_ANON_SECTIONS,
  FORUM_SECTION_LABEL_KEY,
  type ForumViewer,
} from "@/lib/forum";
import { getForumPost } from "@/lib/forum-server";
import { Markdown } from "@/components/markdown";
import { ForumAuthorName, ForumAvatar } from "@/components/forum/ForumAvatar";
import { ForumComments } from "@/components/forum/ForumComments";
import { ForumPostMenu } from "@/components/forum/ForumPostMenu";
import { ForumVote } from "@/components/forum/ForumVote";

interface ForumPostPageProps {
  params: Promise<{ id: string }>;
}

export default async function ForumPostPage({ params }: ForumPostPageProps) {
  const { id } = await params;
  // Fetching the post also records a view for signed-in readers.
  const [post, user] = await Promise.all([getForumPost(id), currentUser()]);
  if (!post) notFound();
  const viewer: ForumViewer | null = user ? { id: user.id, isAdmin: user.role === "admin" } : null;
  const edited = new Date(post.updated_at).getTime() - new Date(post.created_at).getTime() > 60_000;

  return (
    <div className="mx-auto max-w-3xl space-y-6 px-4 py-6 sm:px-6 lg:py-8">
      <Link href={`/forums?section=${post.section}`} className="inline-flex items-center gap-1.5 text-sm text-muted hover:text-ink">
        <ArrowLeft size={15} />
        {t(FORUM_SECTION_LABEL_KEY[post.section])}
      </Link>

      <article className="space-y-5">
        <header className="space-y-4">
          {post.is_pinned && (
            <span className="inline-flex items-center gap-1 text-xs font-medium text-accent">
              <Pin size={12} />
              {t("forum_pinned")}
            </span>
          )}
          <h1 className="text-2xl font-semibold leading-tight tracking-tight sm:text-3xl">{post.title}</h1>
          <div className="flex items-center gap-3">
            <ForumAvatar author={post.author} size={40} />
            <div className="min-w-0 text-sm">
              <ForumAuthorName author={post.author} anonymous={post.is_anonymous && !!post.author} className="font-medium" />
              <div className="flex flex-wrap items-center gap-x-1.5 text-muted">
                <time dateTime={post.created_at} suppressHydrationWarning>
                  {forumTimeAgo(post.created_at)}
                </time>
                {edited && <span>· {t("forum_edited")}</span>}
                <span aria-hidden>·</span>
                <span className="inline-flex items-center gap-1">
                  <Eye size={14} />
                  {compactCount(post.view_count)}
                </span>
              </div>
            </div>
          </div>
          {(post.tags.length > 0 || post.problem_slug) && (
            <div className="flex flex-wrap items-center gap-2">
              {post.problem_slug && (
                <Link
                  href={`/problems/${post.problem_slug}`}
                  className="inline-flex items-center gap-1 rounded-full bg-accent-soft px-2.5 py-0.5 text-xs text-accent hover:opacity-80"
                >
                  <Code2 size={12} />
                  {post.problem_slug}
                </Link>
              )}
              {post.tags.map((tag) => (
                <Link
                  key={tag}
                  href={`/forums?tag=${encodeURIComponent(tag)}`}
                  className="rounded-full bg-surface-2 px-2.5 py-0.5 text-xs text-muted hover:text-ink"
                >
                  #{tag}
                </Link>
              ))}
            </div>
          )}
        </header>

        <Markdown source={post.body_md ?? ""} className="text-[15px] leading-relaxed text-ink" />

        <div className="flex items-center gap-4 border-y border-divider py-3 text-sm text-muted">
          <ForumVote kind="post" id={post.id} score={post.score} myVote={post.my_vote} signedIn={!!viewer} />
          <a href="#comments" className="inline-flex items-center gap-1.5 hover:text-ink">
            <MessageCircle size={16} />
            {compactCount(post.comment_count)}
          </a>
          <span className="ml-auto">
            <ForumPostMenu post={post} viewer={viewer} />
          </span>
        </div>
      </article>

      <ForumComments
        postId={post.id}
        initialCount={post.comment_count}
        allowAnonymous={FORUM_ANON_SECTIONS.includes(post.section)}
        viewer={viewer}
      />
    </div>
  );
}
