"use client";

import { useCallback, useEffect, useState } from "react";
import { ChevronDown, ChevronRight, Edit3, Save, Share2, Trash2, X } from "lucide-react";
import { replyApi, type Note } from "@/features/reply/api";
import { hasErrorCode } from "@/shared/api/errors";
import { formatDateTime } from "@/shared/format";
import { toast } from "@/shared/lib/toast";
import { t } from "@/shared/i18n";

// NotesPanel lists session-private markdown notes attached to the
// candidate's run of this problem. Sharing a note publishes it as a
// comment on the per-problem discussion thread and back-links the note
// via shared_to_comment_id; subsequent share attempts are short-circuited
// by the backend with a 409 conflict.
export function NotesPanel({
  sessionId,
  currentUserId,
}: {
  sessionId: string;
  currentUserId: string;
}) {
  void currentUserId; // reserved for future ownership UI
  const [open, setOpen] = useState(false);
  const [items, setItems] = useState<Note[]>([]);
  const [loading, setLoading] = useState(false);
  const [newBody, setNewBody] = useState("");
  const [editingId, setEditingId] = useState<string | null>(null);
  const [editBody, setEditBody] = useState("");

  const reload = useCallback(async () => {
    if (!sessionId) return;
    setLoading(true);
    try {
      const list = await replyApi.listNotes(sessionId);
      setItems(list);
    } catch (e) {
      toast.error((e as Error).message ?? "Failed to load notes");
    } finally {
      setLoading(false);
    }
  }, [sessionId]);

  useEffect(() => {
    if (open && items.length === 0 && !loading) {
      void reload();
    }
  }, [open, items.length, loading, reload]);

  const onCreate = async () => {
    const trimmed = newBody.trim();
    if (!trimmed) return;
    try {
      await replyApi.createNote(sessionId, trimmed);
      setNewBody("");
      await reload();
    } catch (e) {
      toast.error((e as Error).message ?? "Save failed");
    }
  };

  const beginEdit = (n: Note) => {
    setEditingId(n.id);
    setEditBody(n.body);
  };

  const saveEdit = async (id: string) => {
    const trimmed = editBody.trim();
    if (!trimmed) return;
    try {
      await replyApi.updateNote(id, trimmed);
      setEditingId(null);
      setEditBody("");
      await reload();
    } catch (e) {
      toast.error((e as Error).message ?? "Save failed");
    }
  };

  const cancelEdit = () => {
    setEditingId(null);
    setEditBody("");
  };

  const onDelete = async (id: string) => {
    try {
      await replyApi.deleteNote(id);
      setItems((prev) => prev.filter((n) => n.id !== id));
    } catch (e) {
      toast.error((e as Error).message ?? "Delete failed");
    }
  };

  const onShare = async (id: string) => {
    try {
      await replyApi.shareNote(id);
      toast.success(t("notes_share_success"));
      await reload();
    } catch (e) {
      if (hasErrorCode(e, "already_shared", 409)) {
        toast.info(t("notes_share_success"));
      } else {
        toast.error(t("notes_share_failed"));
      }
    }
  };

  return (
    <div className="mx-auto max-w-[1600px] px-6 py-4 border-t border-divider">
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        className="flex items-center gap-2 text-base font-semibold hover:text-ink"
      >
        {open ? <ChevronDown size={16} /> : <ChevronRight size={16} />}
        {t("notes_title")}
        {items.length > 0 && (
          <span className="text-xs text-muted">({items.length})</span>
        )}
      </button>

      {open && (
        <div className="mt-4 space-y-4">
          {items.length === 0 && !loading && (
            <p className="text-sm text-muted">{t("notes_empty")}</p>
          )}

          <ul className="space-y-2">
            {items.map((n) => (
              <li
                key={n.id}
                className="rounded border border-divider bg-surface p-3"
              >
                {editingId === n.id ? (
                  <div className="space-y-2">
                    <textarea
                      value={editBody}
                      onChange={(e) => setEditBody(e.target.value)}
                      className="w-full min-h-[80px] rounded border border-divider bg-card p-2 text-sm"
                    />
                    <div className="flex items-center gap-2 text-xs">
                      <button
                        type="button"
                        onClick={() => saveEdit(n.id)}
                        disabled={!editBody.trim()}
                        className="flex items-center gap-1 rounded bg-accent px-2 py-1 text-accent-fg disabled:opacity-50"
                      >
                        <Save size={12} />
                        {t("notes_save_btn")}
                      </button>
                      <button
                        type="button"
                        onClick={cancelEdit}
                        className="flex items-center gap-1 rounded border border-divider px-2 py-1 text-muted hover:text-ink"
                      >
                        <X size={12} />
                        {t("cancel")}
                      </button>
                    </div>
                  </div>
                ) : (
                  <div className="space-y-2">
                    <p className="whitespace-pre-wrap text-sm text-ink">
                      {n.body}
                    </p>
                    <div className="flex flex-wrap items-center gap-3 text-xs text-muted">
                      <span className="text-faint">
                        {formatDateTime(n.updated_at)}
                      </span>
                      {n.shared_to_comment_id && (
                        <span className="rounded bg-accent-soft px-1.5 py-0.5 text-[10px] uppercase tracking-wider text-accent">
                          shared
                        </span>
                      )}
                      <button
                        type="button"
                        onClick={() => beginEdit(n)}
                        className="ml-auto flex items-center gap-1 hover:text-ink"
                      >
                        <Edit3 size={12} />
                      </button>
                      <button
                        type="button"
                        onClick={() => onShare(n.id)}
                        className="flex items-center gap-1 hover:text-accent"
                        disabled={!!n.shared_to_comment_id}
                      >
                        <Share2 size={12} />
                        {t("notes_share_btn")}
                      </button>
                      <button
                        type="button"
                        onClick={() => onDelete(n.id)}
                        className="flex items-center gap-1 hover:text-danger"
                      >
                        <Trash2 size={12} />
                      </button>
                    </div>
                  </div>
                )}
              </li>
            ))}
          </ul>

          <div className="border-t border-divider pt-3">
            <textarea
              value={newBody}
              onChange={(e) => setNewBody(e.target.value)}
              placeholder={t("notes_create_placeholder")}
              className="w-full min-h-[64px] rounded border border-divider bg-surface p-2 text-sm"
            />
            <button
              type="button"
              onClick={onCreate}
              disabled={!newBody.trim()}
              className="mt-2 flex items-center gap-1 rounded bg-accent px-3 py-1.5 text-xs font-medium text-accent-fg disabled:opacity-50"
            >
              <Save size={12} />
              {t("notes_save_btn")}
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
