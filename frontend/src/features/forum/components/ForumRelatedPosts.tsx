import Link from "next/link";
import { ArrowBigUp, MessageCircle } from "lucide-react";
import { compactCount } from "@/shared/format";
import { t } from "@/shared/i18n";
import { FORUM_SECTION_LABEL_KEY, type ForumPost } from "@/features/forum/api";

// "Related posts" beside a post: others sharing its tags, problem, or
// section. Sticks alongside the post on wide screens; on narrow ones it
// follows the comments.
export function ForumRelatedPosts({ posts }: { posts: ForumPost[] }) {
  if (posts.length === 0) return null;
  return (
    <aside className="lg:sticky lg:top-20 lg:self-start">
      <section className="rounded-xl border border-divider bg-surface p-4">
        <h2 className="mb-3 text-base font-semibold">{t("forum_related_posts")}</h2>
        <ul className="space-y-3.5">
          {posts.map((p) => (
            <li key={p.id}>
              <Link href={`/forums/${p.id}`} className="line-clamp-2 text-sm leading-snug text-ink hover:text-accent">
                {p.title}
              </Link>
              <div className="mt-1 flex items-center gap-3 text-xs text-faint">
                <span>{t(FORUM_SECTION_LABEL_KEY[p.section])}</span>
                <span className="inline-flex items-center gap-0.5">
                  <ArrowBigUp size={13} />
                  {compactCount(p.score)}
                </span>
                <span className="inline-flex items-center gap-1">
                  <MessageCircle size={12} />
                  {compactCount(p.comment_count)}
                </span>
              </div>
            </li>
          ))}
        </ul>
      </section>
    </aside>
  );
}
