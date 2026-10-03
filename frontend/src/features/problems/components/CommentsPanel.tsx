"use client";

import { useCallback, useEffect, useState } from "react";
import { ChevronDown, ChevronRight } from "lucide-react";
import { problemsApi, type Comment } from "@/features/problems/api";
import { hasErrorCode } from "@/shared/api/errors";
import { UserAvatar } from "@/shared/avatar/UserAvatar";
import { formatDateTime } from "@/shared/format";
import { toast } from "@/shared/lib/toast";
import { t } from "@/shared/i18n";

// CommentsPanel renders the per-problem discussion thread for a graded
// candidate. The list is collapsed by default so the candidate sees the
// reply walkthrough first; opening it kicks off a cursor-paginated fetch.
// Backend returns 403 with `must_complete_problem` when the caller hasn't
// finished the problem, which the panel surfaces inline.
export function CommentsPanel({
  problemSlug,
  currentUserId,
  embedded = false,
}: {
  problemSlug: string;
  currentUserId: string;
  // Embedded: rendered inside a host tab that already titles it, so skip the
  // collapsible header and page padding and load immediately.
  embedded?: boolean;
}) {
  const [open, setOpen] = useState(embedded);
  const [items, setItems] = useState<Comment[]>([]);
  const [cursor, setCursor] = useState("");
  const [hasMore, setHasMore] = useState(false);
  const [loading, setLoading] = useState(false);
  const [body, setBody] = useState("");
  const [gateError, setGateError] = useState<string | null>(null);

  const loadPage = useCallback(
    async (reset: boolean) => {
      if (loading) return;
      setLoading(true);
      try {
        const page = await problemsApi.listComments(
          problemSlug,
          reset ? undefined : cursor,
        );
        setItems((prev) => (reset ? page.comments : [...prev, ...page.comments]));
        setCursor(page.next_cursor);
        setHasMore(!!page.next_cursor);
        setGateError(null);
      } catch (e) {
        const msg = (e as Error).message ?? "";
        if (hasErrorCode(e, "must_complete_problem", 403)) {
          setGateError(t("comments_must_complete"));
        } else {
          toast.error(msg || "Failed to load comments");
        }
      } finally {
        setLoading(false);
      }
    },
    [problemSlug, cursor, loading],
  );

  useEffect(() => {
    if (open && items.length === 0 && !gateError && !loading) {
      void loadPage(true);
    }
  }, [open, items.length, gateError, loading, loadPage]);

  const onPost = async () => {
    const trimmed = body.trim();
    if (!trimmed) return;
    try {
      await problemsApi.createComment(problemSlug, trimmed);
      setBody("");
      setCursor("");
      setItems([]);
      await loadPage(true);
    } catch (e) {
      toast.error((e as Error).message ?? "Post failed");
    }
  };

  const onVote = async (id: string, current: number, next: -1 | 1) => {
    const value: -1 | 0 | 1 = current === next ? 0 : next;
    try {
      const res = await problemsApi.voteComment(id, value);
      setItems((prev) =>
        prev.map((c) =>
          c.id === id
            ? {
                ...c,
                upvotes: res.upvotes,
                downvotes: res.downvotes,
                my_vote: value,
              }
            : c,
        ),
      );
    } catch (e) {
      toast.error((e as Error).message ?? "Vote failed");
    }
  };

  const onDelete = async (id: string) => {
    try {
      await problemsApi.deleteComment(id);
      setItems((prev) => prev.filter((c) => c.id !== id));
    } catch (e) {
      toast.error((e as Error).message ?? "Delete failed");
    }
  };

  return (
    <div className={embedded ? undefined : "mx-auto max-w-[1600px] px-6 py-6 border-t border-divider"}>
      {!embedded && (
        <button
          type="button"
          onClick={() => setOpen((v) => !v)}
          className="flex items-center gap-2 text-base font-semibold hover:text-ink"
        >
          {open ? <ChevronDown size={16} /> : <ChevronRight size={16} />}
          {t("comments_title")}
          {items.length > 0 && (
            <span className="text-xs text-muted">({items.length})</span>
          )}
        </button>
      )}

      {open && (
        <div className={embedded ? "space-y-4" : "mt-4 space-y-4"}>
          {gateError && <p className="text-sm text-muted">{gateError}</p>}

          {!gateError && (
            <>
              {items.length === 0 && !loading && (
                <p className="text-sm text-muted">{t("comments_empty")}</p>
              )}

              <div className="space-y-3">
                {items.map((c) => (
                  <CommentRow
                    key={c.id}
                    comment={c}
                    currentUserId={currentUserId}
                    onVote={onVote}
                    onDelete={onDelete}
                  />
                ))}
              </div>

              {hasMore && (
                <button
                  type="button"
                  onClick={() => loadPage(false)}
                  disabled={loading}
                  className="text-xs text-accent hover:text-ink disabled:opacity-50"
                >
                  {loading ? t("loading") : t("comments_load_more")}
                </button>
              )}

              <div className="border-t border-divider pt-4">
                <textarea
                  value={body}
                  onChange={(e) => setBody(e.target.value)}
                  placeholder={t("comments_post_placeholder")}
                  className="w-full min-h-[64px] rounded border border-divider bg-surface p-2 text-sm"
                />
                <button
                  type="button"
                  onClick={onPost}
                  disabled={!body.trim()}
                  className="mt-2 rounded bg-accent px-3 py-1.5 text-xs font-medium text-accent-fg disabled:opacity-50"
                >
                  {t("comments_post_btn")}
                </button>
              </div>
            </>
          )}
        </div>
      )}
    </div>
  );
}

function CommentRow({
  comment,
  currentUserId,
  onVote,
  onDelete,
}: {
  comment: Comment;
  currentUserId: string;
  onVote: (id: string, current: number, next: -1 | 1) => void;
  onDelete: (id: string) => void;
}) {
  const isMine = comment.user_id === currentUserId;
  return (
    <div className="flex gap-3 border-b border-divider pb-3 last:border-b-0">
      <UserAvatar url={comment.user_avatar_url} seed={comment.user_handle} size={32} />
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2 text-xs text-muted">
          <span className="font-medium text-ink">
            {comment.user_display_name}
          </span>
          <span>@{comment.user_handle}</span>
          {comment.user_tier !== "standard" && (
            <span className="rounded bg-accent-soft px-1.5 py-0.5 text-[10px] uppercase tracking-wider text-accent">
              {comment.user_tier}
            </span>
          )}
          <span className="text-faint">·</span>
          <span className="text-faint">
            {formatDateTime(comment.created_at)}
          </span>
        </div>
        <p className="mt-1 whitespace-pre-wrap text-sm text-ink">
          {comment.body}
        </p>
        <div className="mt-2 flex items-center gap-3 text-xs">
          <button
            type="button"
            onClick={() => onVote(comment.id, comment.my_vote, 1)}
            className={
              comment.my_vote === 1 ? "text-accent" : "text-muted hover:text-ink"
            }
          >
            ▲ {comment.upvotes}
          </button>
          <button
            type="button"
            onClick={() => onVote(comment.id, comment.my_vote, -1)}
            className={
              comment.my_vote === -1
                ? "text-danger"
                : "text-muted hover:text-ink"
            }
          >
            ▼ {comment.downvotes}
          </button>
          {isMine && (
            <button
              type="button"
              onClick={() => onDelete(comment.id)}
              className="ml-auto text-muted hover:text-danger"
            >
              {t("comments_delete_btn")}
            </button>
          )}
        </div>
      </div>
    </div>
  );
}
