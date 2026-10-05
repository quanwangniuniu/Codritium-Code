"use client";

import { useEffect, useRef, useState } from "react";
import { History, X } from "lucide-react";
import { t } from "@/shared/i18n";
import { useLocale } from "@/shared/i18n/client";
import { forumApi, forumErrorMessage, forumTimeAgo, type ForumRevision } from "@/features/forum/api";
import { ForumMarkdown } from "@/features/forum/components/ForumMarkdown";

interface ForumEditHistoryProps {
  postId: string;
  updatedAt: string;
  // The author and moderators can open earlier versions.
  canView: boolean;
}

// "· edited 3h ago" on a post, with the exact time on hover; for those who
// may see it, a History button opens the earlier versions.
export function ForumEditHistory({ postId, updatedAt, canView }: ForumEditHistoryProps) {
  useLocale();
  const [open, setOpen] = useState(false);
  return (
    <>
      <span title={new Date(updatedAt).toLocaleString()} suppressHydrationWarning>
        · {t("forum_edited")} {forumTimeAgo(updatedAt)}
      </span>
      {canView && (
        <button
          type="button"
          onClick={() => setOpen(true)}
          className="inline-flex items-center gap-0.5 text-accent hover:underline"
        >
          <History size={12} />
          {t("forum_history_view")}
        </button>
      )}
      {open && <HistoryDialog postId={postId} onClose={() => setOpen(false)} />}
    </>
  );
}

function HistoryDialog({ postId, onClose }: { postId: string; onClose: () => void }) {
  const ref = useRef<HTMLDialogElement>(null);
  const [revisions, setRevisions] = useState<ForumRevision[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const el = ref.current;
    if (el && !el.open) el.showModal();
    let cancelled = false;
    forumApi
      .revisions(postId)
      .then((r) => !cancelled && setRevisions(r.revisions))
      .catch((e) => !cancelled && setError(forumErrorMessage(e)));
    return () => {
      cancelled = true;
      el?.close();
    };
  }, [postId]);

  return (
    <dialog
      ref={ref}
      onCancel={(e) => {
        e.preventDefault();
        onClose();
      }}
      onClick={(e) => e.target === e.currentTarget && onClose()}
      aria-labelledby="forum-history-title"
      className="m-auto max-h-[80vh] w-[min(40rem,calc(100vw-2rem))] overflow-hidden rounded-xl border border-divider bg-surface p-0 text-ink shadow-xl backdrop:bg-black/40"
    >
      <div className="flex items-center justify-between border-b border-divider px-5 py-3">
        <h2 id="forum-history-title" className="text-base font-semibold">
          {t("forum_history_title")}
        </h2>
        <button type="button" onClick={onClose} aria-label={t("cancel")} className="text-muted hover:text-ink">
          <X size={18} />
        </button>
      </div>
      <div className="max-h-[calc(80vh-3.5rem)] space-y-3 overflow-y-auto p-5">
        {error ? (
          <p className="text-sm text-danger">{error}</p>
        ) : revisions === null ? (
          <p className="text-sm text-muted">{t("loading")}</p>
        ) : revisions.length === 0 ? (
          <p className="text-sm text-muted">{t("forum_history_empty")}</p>
        ) : (
          revisions.map((rev, i) => (
            <details key={rev.replaced_at} open={i === 0} className="rounded-lg border border-divider">
              <summary className="cursor-pointer px-4 py-2.5 text-sm">
                <span className="font-medium">{rev.title}</span>
                <span className="ml-2 text-xs text-muted" suppressHydrationWarning>
                  {t("forum_history_replaced_fmt", { params: { when: forumTimeAgo(rev.replaced_at) } })}
                </span>
              </summary>
              <div className="border-t border-divider px-4 py-3">
                <ForumMarkdown source={rev.body_md} compact />
                {rev.tags.length > 0 && (
                  <p className="mt-2 text-xs text-faint">{rev.tags.map((tag) => `#${tag}`).join(" ")}</p>
                )}
              </div>
            </details>
          ))
        )}
      </div>
    </dialog>
  );
}
