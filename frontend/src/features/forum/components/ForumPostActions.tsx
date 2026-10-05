"use client";

import { useState } from "react";
import { Bell, BellRing, Bookmark, BookmarkCheck } from "lucide-react";
import { t } from "@/shared/i18n";
import { useLocale } from "@/shared/i18n/client";
import { toast } from "@/shared/lib/toast";
import { cn } from "@/shared/lib/cn";
import { forumApi, forumErrorMessage, type ForumPost } from "@/features/forum/api";

// Save (bookmark) and Follow toggles for a signed-in reader. Both update
// optimistically and roll back if the request fails.
export function ForumPostActions({ post }: { post: Pick<ForumPost, "id" | "is_bookmarked" | "is_following"> }) {
  useLocale();
  const [saved, setSaved] = useState(post.is_bookmarked);
  const [following, setFollowing] = useState(post.is_following);

  async function toggle(
    current: boolean,
    set: (v: boolean) => void,
    call: (next: boolean) => Promise<unknown>,
    messages: [on: string, off: string],
  ) {
    const next = !current;
    set(next);
    try {
      await call(next);
      toast.success(next ? messages[0] : messages[1]);
    } catch (e) {
      set(current);
      toast.error(forumErrorMessage(e));
    }
  }

  const button = "inline-flex items-center gap-1.5 rounded-md px-2 py-1 transition-colors hover:bg-surface-2 hover:text-ink";

  return (
    <>
      <button
        type="button"
        aria-pressed={saved}
        onClick={() =>
          void toggle(saved, setSaved, (v) => forumApi.bookmarkPost(post.id, v), [t("forum_saved_toast"), t("forum_unsaved_toast")])
        }
        className={cn(button, saved && "text-accent")}
      >
        {saved ? <BookmarkCheck size={16} /> : <Bookmark size={16} />}
        {saved ? t("forum_saved") : t("forum_save_post")}
      </button>
      <button
        type="button"
        aria-pressed={following}
        title={t("forum_follow_hint")}
        onClick={() =>
          void toggle(following, setFollowing, (v) => forumApi.followPost(post.id, v), [
            t("forum_followed_toast"),
            t("forum_unfollowed_toast"),
          ])
        }
        className={cn(button, following && "text-accent")}
      >
        {following ? <BellRing size={16} /> : <Bell size={16} />}
        {following ? t("forum_following") : t("forum_follow")}
      </button>
    </>
  );
}
