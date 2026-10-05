"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { MessageCircle, Pencil, Reply, Trash2 } from "lucide-react";
import { t } from "@/shared/i18n";
import { useLocale } from "@/shared/i18n/client";
import { toast } from "@/shared/lib/toast";
import { cn } from "@/shared/lib/cn";
import {
  FORUM_LIMITS,
  forumApi,
  forumErrorMessage,
  forumTimeAgo,
  type ForumComment,
  type ForumCommentSort,
  type ForumViewer,
} from "@/features/forum/api";
import { ForumMarkdown } from "@/features/forum/components/ForumMarkdown";
import { ForumAuthorName, ForumAvatar } from "@/features/forum/components/ForumAvatar";
import { ForumVote } from "@/features/forum/components/ForumVote";

interface ForumCommentsProps {
  postId: string;
  initialCount: number;
  allowAnonymous: boolean;
  viewer: ForumViewer | null;
}

export function ForumComments({ postId, initialCount, allowAnonymous, viewer }: ForumCommentsProps) {
  useLocale();
  const [sort, setSort] = useState<ForumCommentSort>("best");
  const [comments, setComments] = useState<ForumComment[]>([]);
  const [hasMore, setHasMore] = useState(false);
  const [offset, setOffset] = useState(0);
  const [loading, setLoading] = useState(true);
  const [count, setCount] = useState(initialCount);
  const [replyTo, setReplyTo] = useState<string | null>(null);

  const load = useCallback(
    async (nextSort: ForumCommentSort, from: number) => {
      setLoading(true);
      try {
        const page = await forumApi.listComments(postId, nextSort, from);
        setComments((prev) => (from === 0 ? page.comments : [...prev, ...page.comments]));
        setOffset(from + page.comments.length);
        setHasMore(page.has_more);
      } catch (e) {
        toast.error(forumErrorMessage(e));
      } finally {
        setLoading(false);
      }
    },
    [postId],
  );

  useEffect(() => {
    void load(sort, 0);
  }, [load, sort]);

  function added(c: ForumComment) {
    setCount((n) => n + 1);
    if (c.parent_id) {
      setComments((prev) =>
        prev.map((p) => (p.id === c.parent_id ? { ...p, replies: [...(p.replies ?? []), c] } : p)),
      );
      setReplyTo(null);
    } else {
      // Top-level comments show first until the next reload re-sorts them;
      // the server offset moves along with them.
      setComments((prev) => [c, ...prev]);
      setOffset((o) => o + 1);
    }
  }

  // Mirrors the server: a deleted top-level comment with replies becomes a
  // "[deleted]" placeholder, and a placeholder goes once its last reply does.
  async function remove(c: ForumComment) {
    if (!window.confirm(t("forum_delete_comment_confirm"))) return;
    try {
      await forumApi.deleteComment(c.id);
      setCount((n) => Math.max(0, n - 1));
      const parent = comments.find((p) => p.id === (c.parent_id ?? c.id));
      const remaining = (parent?.replies ?? []).filter((r) => r.id !== c.id).length;
      // The top-level comment leaves the list (and the server's paging) when it
      // has no replies left, or when its last reply goes and it was deleted.
      const dropsTop = c.parent_id ? !!parent?.is_deleted && remaining === 0 : remaining === 0;
      setComments((prev) =>
        prev.flatMap((p) => {
          if (p.id !== parent?.id) return [p];
          if (dropsTop) return [];
          if (c.parent_id) return [{ ...p, replies: (p.replies ?? []).filter((r) => r.id !== c.id) }];
          return [{ ...p, is_deleted: true, body: "", author: null, is_mine: false, is_op: false, my_vote: 0 as const }];
        }),
      );
      if (dropsTop) setOffset((o) => Math.max(0, o - 1));
      toast.success(t("forum_comment_deleted"));
    } catch (e) {
      toast.error(forumErrorMessage(e));
    }
  }

  function edited(c: ForumComment) {
    setComments((prev) =>
      prev.map((p) => {
        if (p.id === c.id) return { ...c, replies: p.replies };
        if (c.parent_id && p.id === c.parent_id) {
          return { ...p, replies: (p.replies ?? []).map((r) => (r.id === c.id ? c : r)) };
        }
        return p;
      }),
    );
  }

  return (
    <section id="comments" className="scroll-mt-20 space-y-5">
      <div className="flex items-center justify-between gap-4">
        <h2 className="flex items-center gap-2 text-lg font-semibold">
          <MessageCircle size={18} />
          {t("forum_comments_count_fmt", { params: { n: count } })}
        </h2>
        <div className="flex gap-1 text-sm" role="group" aria-label={t("forum_sort_comments")}>
          {(["best", "newest"] as const).map((s) => (
            <button
              key={s}
              type="button"
              aria-pressed={sort === s}
              onClick={() => setSort(s)}
              className={cn(
                "rounded-md px-2.5 py-1 transition-colors",
                sort === s ? "bg-surface-2 font-medium text-ink" : "text-muted hover:text-ink",
              )}
            >
              {s === "best" ? t("forum_sort_best") : t("forum_sort_newest")}
            </button>
          ))}
        </div>
      </div>

      {viewer ? (
        <CommentComposer postId={postId} parentId={null} allowAnonymous={allowAnonymous} onPosted={added} />
      ) : (
        <p className="rounded-md border border-divider bg-surface p-4 text-sm text-muted">
          <Link href={`/login?next=${encodeURIComponent(`/forums/${postId}`)}`} className="text-accent underline">
            {t("forum_login_link")}
          </Link>{" "}
          {t("forum_login_to_comment")}
        </p>
      )}

      {loading && comments.length === 0 ? (
        <p className="py-6 text-center text-sm text-muted">{t("loading")}</p>
      ) : comments.length === 0 ? (
        <p className="py-6 text-center text-sm text-muted">{t("forum_no_comments")}</p>
      ) : (
        <ul className="divide-y divide-divider">
          {comments.map((c) => (
            <li key={c.id} className="py-4">
              <CommentBody
                comment={c}
                viewer={viewer}
                onReply={viewer && !c.is_deleted ? () => setReplyTo(replyTo === c.id ? null : c.id) : undefined}
                onDelete={() => remove(c)}
                onEdited={edited}
              />
              {((c.replies?.length ?? 0) > 0 || replyTo === c.id) && (
                <div className="ml-11 mt-3 space-y-4 border-l border-divider pl-4">
                  {c.replies?.map((r) => (
                    <CommentBody key={r.id} comment={r} viewer={viewer} onDelete={() => remove(r)} onEdited={edited} />
                  ))}
                  {replyTo === c.id && (
                    <CommentComposer
                      postId={postId}
                      parentId={c.id}
                      allowAnonymous={allowAnonymous}
                      onPosted={added}
                      onCancel={() => setReplyTo(null)}
                      autoFocus
                    />
                  )}
                </div>
              )}
            </li>
          ))}
        </ul>
      )}

      {hasMore && (
        <div className="flex justify-center">
          <button
            type="button"
            disabled={loading}
            onClick={() => void load(sort, offset)}
            className="rounded-md border border-divider px-4 py-2 text-sm text-muted hover:text-ink disabled:opacity-60"
          >
            {loading ? t("loading") : t("forum_load_more_comments")}
          </button>
        </div>
      )}
    </section>
  );
}

function CommentBody({
  comment,
  viewer,
  onReply,
  onDelete,
  onEdited,
}: {
  comment: ForumComment;
  viewer: ForumViewer | null;
  onReply?: () => void;
  onDelete: () => void;
  onEdited: (c: ForumComment) => void;
}) {
  const [editing, setEditing] = useState(false);

  if (comment.is_deleted) {
    return (
      <div className="flex gap-3">
        <div className="h-8 w-8 shrink-0 rounded-full bg-surface-2" aria-hidden />
        <p className="py-1.5 text-sm italic text-faint">{t("forum_deleted_comment")}</p>
      </div>
    );
  }

  const canDelete = comment.is_mine || viewer?.isAdmin;
  return (
    <div className="flex gap-3">
      <ForumAvatar author={comment.author} size={32} />
      <div className="min-w-0 flex-1">
        <div className="flex flex-wrap items-center gap-x-1.5 text-sm">
          <ForumAuthorName author={comment.author} anonymous={comment.is_anonymous && !!comment.author} className="font-medium" />
          {comment.is_op && (
            <span className="rounded bg-accent-soft px-1.5 py-px text-[11px] font-medium text-accent">
              {t("forum_op_badge")}
            </span>
          )}
          <span className="text-muted" aria-hidden>
            ·
          </span>
          <time dateTime={comment.created_at} className="text-muted" suppressHydrationWarning>
            {forumTimeAgo(comment.created_at)}
          </time>
          {comment.edited_at && <span className="text-muted">· {t("forum_edited")}</span>}
        </div>
        {editing ? (
          <div className="mt-2">
            <CommentEditor
              comment={comment}
              onSaved={(c) => {
                setEditing(false);
                onEdited(c);
              }}
              onCancel={() => setEditing(false)}
            />
          </div>
        ) : (
          <>
            <ForumMarkdown source={comment.body} compact className="mt-1" />
            <div className="mt-1 flex items-center gap-3 text-xs text-muted">
              <ForumVote
                kind="comment"
                id={comment.id}
                score={comment.score}
                myVote={comment.my_vote}
                signedIn={!!viewer}
                size="sm"
              />
              {onReply && (
                <button type="button" onClick={onReply} className="inline-flex items-center gap-1 hover:text-ink">
                  <Reply size={14} />
                  {t("forum_reply")}
                </button>
              )}
              {comment.is_mine && (
                <button type="button" onClick={() => setEditing(true)} className="inline-flex items-center gap-1 hover:text-ink">
                  <Pencil size={13} />
                  {t("forum_edit")}
                </button>
              )}
              {canDelete && (
                <button type="button" onClick={onDelete} className="inline-flex items-center gap-1 hover:text-danger">
                  <Trash2 size={13} />
                  {t("forum_delete")}
                </button>
              )}
            </div>
          </>
        )}
      </div>
    </div>
  );
}

function CommentEditor({
  comment,
  onSaved,
  onCancel,
}: {
  comment: ForumComment;
  onSaved: (c: ForumComment) => void;
  onCancel: () => void;
}) {
  const [body, setBody] = useState(comment.body);
  const [busy, setBusy] = useState(false);
  const trimmed = body.trim();

  async function save() {
    if (!trimmed || busy) return;
    if (trimmed === comment.body) {
      onCancel();
      return;
    }
    setBusy(true);
    try {
      onSaved(await forumApi.updateComment(comment.id, trimmed));
      toast.success(t("forum_comment_updated"));
    } catch (e) {
      toast.error(forumErrorMessage(e));
      setBusy(false);
    }
  }

  return (
    <div className="space-y-2">
      <textarea
        value={body}
        onChange={(e) => setBody(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) void save();
          if (e.key === "Escape") onCancel();
        }}
        maxLength={FORUM_LIMITS.commentMax}
        autoFocus
        rows={3}
        aria-label={t("forum_edit")}
        className="w-full resize-y rounded-md border border-divider bg-surface p-3 text-sm text-ink outline-none focus:border-accent"
      />
      <div className="flex justify-end gap-2">
        <button type="button" onClick={onCancel} className="rounded-md px-3 py-1.5 text-sm text-muted hover:text-ink">
          {t("cancel")}
        </button>
        <button
          type="button"
          onClick={() => void save()}
          disabled={!trimmed || busy}
          className="rounded-md bg-accent px-3.5 py-1.5 text-sm font-medium text-accent-fg disabled:opacity-50"
        >
          {busy ? t("loading") : t("forum_save")}
        </button>
      </div>
    </div>
  );
}

function CommentComposer({
  postId,
  parentId,
  allowAnonymous,
  onPosted,
  onCancel,
  autoFocus,
}: {
  postId: string;
  parentId: string | null;
  allowAnonymous: boolean;
  onPosted: (c: ForumComment) => void;
  onCancel?: () => void;
  autoFocus?: boolean;
}) {
  const [body, setBody] = useState("");
  const [anonymous, setAnonymous] = useState(false);
  const [busy, setBusy] = useState(false);
  const trimmed = body.trim();

  async function submit() {
    if (!trimmed || busy) return;
    setBusy(true);
    try {
      const c = await forumApi.createComment(postId, trimmed, parentId, anonymous);
      setBody("");
      onPosted(c);
    } catch (e) {
      toast.error(forumErrorMessage(e));
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="space-y-2">
      <textarea
        value={body}
        onChange={(e) => setBody(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) void submit();
        }}
        placeholder={parentId ? t("forum_reply_placeholder") : t("forum_comment_placeholder")}
        maxLength={FORUM_LIMITS.commentMax}
        autoFocus={autoFocus}
        rows={parentId ? 2 : 3}
        className="w-full resize-y rounded-md border border-divider bg-surface p-3 text-sm text-ink outline-none placeholder:text-faint focus:border-accent"
      />
      <div className="flex flex-wrap items-center gap-3">
        {allowAnonymous && (
          <label className="inline-flex items-center gap-2 text-xs text-muted">
            <input type="checkbox" checked={anonymous} onChange={(e) => setAnonymous(e.target.checked)} />
            {t("forum_comment_anonymously")}
          </label>
        )}
        <div className="ml-auto flex gap-2">
          {onCancel && (
            <button type="button" onClick={onCancel} className="rounded-md px-3 py-1.5 text-sm text-muted hover:text-ink">
              {t("cancel")}
            </button>
          )}
          <button
            type="button"
            onClick={() => void submit()}
            disabled={!trimmed || busy}
            className="rounded-md bg-accent px-3.5 py-1.5 text-sm font-medium text-accent-fg disabled:opacity-50"
          >
            {parentId ? t("forum_reply") : t("forum_post_comment")}
          </button>
        </div>
      </div>
    </div>
  );
}
