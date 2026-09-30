import { BadgeCheck, UserRound } from "lucide-react";
import { t } from "@/lib/i18n";
import { cn } from "@/lib/utils";
import { initialOf } from "@/shared/format";
import type { ForumAuthor } from "@/lib/forum";

const FALLBACK_COLORS = ["#7dd3fc", "#a7f3d0", "#fcd34d", "#f9a8d4", "#c4b5fd", "#fdba74"];

function fallbackColor(seed: string): string {
  let h = 0;
  for (const ch of seed) h = (h * 31 + ch.charCodeAt(0)) >>> 0;
  return FALLBACK_COLORS[h % FALLBACK_COLORS.length];
}

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
  if (author.avatar_url) {
    // eslint-disable-next-line @next/next/no-img-element
    return <img src={author.avatar_url} alt="" style={style} className="shrink-0 rounded-full object-cover" />;
  }
  return (
    <span
      style={{ ...style, background: author.avatar_color || fallbackColor(author.handle) }}
      className="grid shrink-0 place-items-center rounded-full text-xs font-semibold text-[#0b1220]"
      aria-hidden
    >
      {initialOf(author.display_name, author.handle)}
    </span>
  );
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
      {/* Visible only to the author and admins: the post is anonymous to everyone else. */}
      {anonymous && <span className="text-xs text-faint">({t("forum_posted_anonymously")})</span>}
    </span>
  );
}
