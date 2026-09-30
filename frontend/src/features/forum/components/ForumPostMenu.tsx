"use client";

import { useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { Ellipsis, Link2, Pencil, Pin, PinOff, Trash2 } from "lucide-react";
import { t } from "@/shared/i18n";
import { useLocale } from "@/shared/i18n/client";
import { toast } from "@/shared/lib/toast";
import { cn } from "@/shared/lib/cn";
import { forumApi, forumErrorMessage, type ForumPost, type ForumViewer } from "@/features/forum/api";

interface ForumPostMenuProps {
  post: Pick<ForumPost, "id" | "is_mine" | "is_pinned">;
  viewer: ForumViewer | null;
  onDeleted?: () => void;
}

// The "…" menu on a post: copy link for everyone, edit for the author,
// delete for the author or an admin, pin/unpin for admins.
export function ForumPostMenu({ post, viewer, onDeleted }: ForumPostMenuProps) {
  useLocale();
  const router = useRouter();
  const [open, setOpen] = useState(false);
  const [busy, setBusy] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    const onDown = (e: MouseEvent) => {
      if (!ref.current?.contains(e.target as Node)) setOpen(false);
    };
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && setOpen(false);
    window.addEventListener("mousedown", onDown);
    window.addEventListener("keydown", onKey);
    return () => {
      window.removeEventListener("mousedown", onDown);
      window.removeEventListener("keydown", onKey);
    };
  }, [open]);

  const canDelete = post.is_mine || viewer?.isAdmin;

  async function copyLink() {
    setOpen(false);
    try {
      await navigator.clipboard.writeText(`${window.location.origin}/forums/${post.id}`);
      toast.success(t("forum_link_copied"));
    } catch {
      toast.error(t("forum_err_generic"));
    }
  }

  async function remove() {
    setOpen(false);
    if (!window.confirm(t("forum_delete_post_confirm"))) return;
    setBusy(true);
    try {
      await forumApi.deletePost(post.id);
      toast.success(t("forum_post_deleted"));
      if (onDeleted) onDeleted();
      else router.push("/forums");
    } catch (e) {
      toast.error(forumErrorMessage(e));
    } finally {
      setBusy(false);
    }
  }

  async function togglePin() {
    setOpen(false);
    setBusy(true);
    try {
      await forumApi.pinPost(post.id, !post.is_pinned);
      toast.success(post.is_pinned ? t("forum_post_unpinned") : t("forum_post_pinned"));
      router.refresh();
    } catch (e) {
      toast.error(forumErrorMessage(e));
    } finally {
      setBusy(false);
    }
  }

  const item =
    "flex w-full items-center gap-2 px-3 py-2 text-left text-sm text-ink hover:bg-surface-2";

  return (
    <div ref={ref} className="relative">
      <button
        type="button"
        aria-label={t("forum_post_actions")}
        aria-haspopup="menu"
        aria-expanded={open}
        disabled={busy}
        onClick={(e) => {
          e.preventDefault();
          setOpen((v) => !v);
        }}
        className="grid h-8 w-8 place-items-center rounded-md text-muted hover:bg-surface-2 hover:text-ink disabled:opacity-50"
      >
        <Ellipsis size={18} />
      </button>
      {open && (
        <div
          role="menu"
          className="absolute right-0 z-20 mt-1 w-44 overflow-hidden rounded-md border border-divider bg-surface py-1 shadow-lg"
        >
          <button type="button" role="menuitem" className={item} onClick={copyLink}>
            <Link2 size={14} />
            {t("forum_copy_link")}
          </button>
          {post.is_mine && (
            <button
              type="button"
              role="menuitem"
              className={item}
              onClick={() => router.push(`/forums/${post.id}/edit`)}
            >
              <Pencil size={14} />
              {t("forum_edit")}
            </button>
          )}
          {viewer?.isAdmin && (
            <button type="button" role="menuitem" className={item} onClick={togglePin}>
              {post.is_pinned ? <PinOff size={14} /> : <Pin size={14} />}
              {post.is_pinned ? t("forum_unpin") : t("forum_pin")}
            </button>
          )}
          {canDelete && (
            <button type="button" role="menuitem" className={cn(item, "text-danger")} onClick={remove}>
              <Trash2 size={14} />
              {t("forum_delete")}
            </button>
          )}
        </div>
      )}
    </div>
  );
}
