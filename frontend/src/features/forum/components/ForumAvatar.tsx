import { BadgeCheck, UserRound } from "lucide-react";
import { t } from "@/shared/i18n";
import { compactCount } from "@/shared/format";
import { cn } from "@/shared/lib/cn";
import { UserAvatar } from "@/shared/avatar/UserAvatar";
import type { ForumAuthor } from "@/features/forum/api";

// Author avatar; `null` renders the anonymous placeholder.
export function ForumAvatar({
  author,
  size = 36,
}: {
  author: ForumAuthor | null;
  size?: number;
}) {
  const style = { width: size, height: size };
  if (!author) {
    return (
      <span
        style={style}
        className="grid shrink-0 place-items-center rounded-full bg-surface-2 text-faint"
        aria-hidden
      >
        <UserRound size={Math.round(size * 0.55)} />
      </span>
    );
  }
  return <UserAvatar url={author.avatar_url} seed={author.handle} size={size} />;
}

// "Name ✓" or "Anonymous", used on post cards, post pages and comments.
export function ForumAuthorName({
  author,
  anonymous,
  className,
}: {
  author: ForumAuthor | null;
  anonymous: boolean;
  className?: string;
}) {
  if (!author) {
    return <span className={cn("text-muted", className)}>{t("forum_anonymous")}</span>;
  }
  return (
    <span className={cn("inline-flex items-center gap-1 text-ink", className)}>
      {author.display_name || author.handle}
      {author.verified && (
        <BadgeCheck size={15} className="text-accent" aria-label={t("forum_verified")}>
          <title>{t("forum_verified")}</title>
        </BadgeCheck>
      )}
      {author.reputation > 0 && (
        <span
          className="rounded bg-surface-2 px-1 text-[11px] font-normal tabular-nums text-muted"
          title={t("forum_reputation_fmt", { params: { n: author.reputation } })}
        >
          {compactCount(author.reputation)}
        </span>
      )}
      {/* Visible only to the author and admins: the post is anonymous to everyone else. */}
      {anonymous && <span className="text-xs text-faint">({t("forum_posted_anonymously")})</span>}
    </span>
  );
}
