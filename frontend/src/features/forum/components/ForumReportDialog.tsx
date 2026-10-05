"use client";

import { useEffect, useRef, useState } from "react";
import { X } from "lucide-react";
import { t, type LocaleKey } from "@/shared/i18n";
import { useLocale } from "@/shared/i18n/client";
import { toast } from "@/shared/lib/toast";
import { forumApi, forumErrorMessage, REPORT_REASONS, type ReportReason } from "@/features/forum/api";

const REASON_KEY: Record<ReportReason, LocaleKey> = {
  spam: "forum_report_reason_spam",
  abuse: "forum_report_reason_abuse",
  off_topic: "forum_report_reason_off_topic",
  other: "forum_report_reason_other",
};

interface ForumReportDialogProps {
  postId: string;
  // Set to report a comment rather than the post.
  commentId?: string;
  onClose: () => void;
}

// Modal for flagging a post or comment: pick a reason, optionally explain.
// Rendered only while open; Escape or the backdrop closes it.
export function ForumReportDialog({ postId, commentId, onClose }: ForumReportDialogProps) {
  useLocale();
  const ref = useRef<HTMLDialogElement>(null);
  const [reason, setReason] = useState<ReportReason | null>(null);
  const [details, setDetails] = useState("");
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    const el = ref.current;
    if (el && !el.open) el.showModal();
    return () => el?.close();
  }, []);

  async function submit() {
    if (!reason || busy) return;
    setBusy(true);
    try {
      await forumApi.report({ postId, commentId }, reason, details.trim());
      toast.success(t("forum_report_sent"));
      onClose();
    } catch (e) {
      toast.error(forumErrorMessage(e));
      setBusy(false);
    }
  }

  return (
    <dialog
      ref={ref}
      onCancel={(e) => {
        e.preventDefault();
        onClose();
      }}
      onClick={(e) => e.target === e.currentTarget && onClose()}
      className="m-auto w-[min(28rem,calc(100vw-2rem))] rounded-xl border border-divider bg-surface p-0 text-ink shadow-xl backdrop:bg-black/40"
    >
      <form
        method="dialog"
        className="space-y-4 p-5"
        onSubmit={(e) => {
          e.preventDefault();
          void submit();
        }}
      >
        <div className="flex items-center justify-between">
          <h2 className="text-base font-semibold">
            {commentId ? t("forum_report_title_comment") : t("forum_report_title_post")}
          </h2>
          <button type="button" onClick={onClose} aria-label={t("cancel")} className="text-muted hover:text-ink">
            <X size={18} />
          </button>
        </div>
        <fieldset className="space-y-2">
          {REPORT_REASONS.map((r) => (
            <label key={r} className="flex cursor-pointer items-center gap-2.5 rounded-md border border-divider px-3 py-2 text-sm has-[:checked]:border-accent">
              <input type="radio" name="reason" value={r} checked={reason === r} onChange={() => setReason(r)} />
              {t(REASON_KEY[r])}
            </label>
          ))}
        </fieldset>
        <textarea
          value={details}
          onChange={(e) => setDetails(e.target.value)}
          maxLength={500}
          rows={3}
          placeholder={t("forum_report_details_placeholder")}
          className="w-full resize-y rounded-md border border-divider bg-surface p-3 text-sm outline-none placeholder:text-faint focus:border-accent"
        />
        <div className="flex justify-end gap-2">
          <button type="button" onClick={onClose} className="rounded-md px-3 py-1.5 text-sm text-muted hover:text-ink">
            {t("cancel")}
          </button>
          <button
            type="submit"
            disabled={!reason || busy}
            className="rounded-md bg-danger px-3.5 py-1.5 text-sm font-medium text-white disabled:opacity-50"
          >
            {busy ? t("loading") : t("forum_report_submit")}
          </button>
        </div>
      </form>
    </dialog>
  );
}
