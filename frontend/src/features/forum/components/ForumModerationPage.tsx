"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { Ban, Check, Flag, MessageSquare, Trash2, FileText } from "lucide-react";
import { t, type LocaleKey } from "@/shared/i18n";
import { useLocale } from "@/shared/i18n/client";
import { toast } from "@/shared/lib/toast";
import { cn } from "@/shared/lib/cn";
import { formatShortDate } from "@/shared/format";
import {
  forumAdminApi,
  forumErrorMessage,
  forumTimeAgo,
  type ForumMute,
  type ReportReason,
  type ReportedItem,
} from "@/features/forum/api";

const REASON_KEY: Record<ReportReason, LocaleKey> = {
  spam: "forum_report_reason_spam",
  abuse: "forum_report_reason_abuse",
  off_topic: "forum_report_reason_off_topic",
  other: "forum_report_reason_other",
};

const MUTE_DAYS = [1, 3, 7, 30];

function daysLabel(n: number): string {
  return n === 1 ? t("mod_day_one") : t("mod_days_fmt", { params: { n } });
}

type Tab = "reports" | "mutes";

// /admin/forum: the report queue and the list of muted users.
export function ForumModerationPage() {
  useLocale();
  const [tab, setTab] = useState<Tab>("reports");

  return (
    <div className="mx-auto max-w-3xl space-y-5 px-4 py-6 sm:px-6 lg:py-8">
      <div className="flex flex-wrap items-center gap-3">
        <h1 className="text-xl font-semibold">{t("mod_title")}</h1>
        <div className="ml-auto flex gap-1 text-sm" role="tablist">
          {(["reports", "mutes"] as const).map((v) => (
            <button
              key={v}
              type="button"
              role="tab"
              aria-selected={tab === v}
              onClick={() => setTab(v)}
              className={cn(
                "rounded-md px-3 py-1.5 transition-colors",
                tab === v ? "bg-surface-2 font-medium text-ink" : "text-muted hover:text-ink",
              )}
            >
              {v === "reports" ? t("mod_tab_reports") : t("mod_tab_mutes")}
            </button>
          ))}
        </div>
      </div>
      {tab === "reports" ? <ReportQueue /> : <MuteList />}
    </div>
  );
}

function ReportQueue() {
  const [items, setItems] = useState<ReportedItem[] | null>(null);
  const [hasMore, setHasMore] = useState(false);
  const [open, setOpen] = useState(0);

  const load = useCallback(async (offset: number) => {
    try {
      const page = await forumAdminApi.reports(offset);
      setItems((prev) => (offset === 0 || !prev ? page.items : [...prev, ...page.items]));
      setHasMore(page.has_more);
      setOpen(page.open_targets);
    } catch (e) {
      toast.error(forumErrorMessage(e));
      setItems((prev) => prev ?? []);
    }
  }, []);

  useEffect(() => {
    void load(0);
  }, [load]);

  async function resolve(it: ReportedItem, action: "remove" | "dismiss", muteDays = 0) {
    if (action === "remove" && !window.confirm(t("mod_remove_confirm"))) return;
    try {
      await forumAdminApi.resolve(it, action, muteDays, it.reasons.join(", "));
      setItems((prev) => prev?.filter((x) => x !== it) ?? null);
      setOpen((n) => Math.max(0, n - 1));
      toast.success(
        action === "dismiss"
          ? t("mod_dismissed")
          : muteDays > 0
            ? t("mod_removed_muted_fmt", { params: { days: daysLabel(muteDays) } })
            : t("mod_removed"),
      );
    } catch (e) {
      toast.error(forumErrorMessage(e));
    }
  }

  if (items === null) return <p className="py-12 text-center text-sm text-muted">{t("loading")}</p>;
  if (items.length === 0) {
    return (
      <div className="flex flex-col items-center gap-3 rounded-xl border border-divider bg-surface py-14 text-center">
        <Check size={28} className="text-success" />
        <p className="text-sm text-muted">{t("mod_queue_empty")}</p>
      </div>
    );
  }

  return (
    <div className="space-y-3">
      <p className="text-sm text-muted">{open === 1 ? t("mod_open_one") : t("mod_open_fmt", { params: { n: open } })}</p>
      {items.map((it) => (
        <ReportCard key={`${it.post_id}:${it.comment_id ?? ""}`} it={it} onResolve={resolve} />
      ))}
      {hasMore && (
        <div className="flex justify-center">
          <button
            type="button"
            onClick={() => void load(items.length)}
            className="rounded-md border border-divider px-4 py-2 text-sm text-muted hover:text-ink"
          >
            {t("forum_load_more")}
          </button>
        </div>
      )}
    </div>
  );
}

function ReportCard({
  it,
  onResolve,
}: {
  it: ReportedItem;
  onResolve: (it: ReportedItem, action: "remove" | "dismiss", muteDays?: number) => void;
}) {
  const [muteDays, setMuteDays] = useState(MUTE_DAYS[0]);
  const href = it.comment_id ? `/forums/${it.post_id}#comment-${it.comment_id}` : `/forums/${it.post_id}`;
  const button = "inline-flex items-center gap-1.5 rounded-md border border-divider px-2.5 py-1.5 text-sm transition-colors";

  return (
    <article className="space-y-3 rounded-xl border border-divider bg-surface p-4">
      <div className="flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-muted">
        <span className="inline-flex items-center gap-1 rounded bg-surface-2 px-1.5 py-0.5 font-medium text-ink">
          {it.comment_id ? <MessageSquare size={12} /> : <FileText size={12} />}
          {it.comment_id ? t("mod_kind_comment") : t("mod_kind_post")}
        </span>
        <span className="inline-flex items-center gap-1 font-medium text-danger">
          <Flag size={12} />
          {it.reports === 1 ? t("mod_report_one") : t("mod_reports_fmt", { params: { n: it.reports } })}
        </span>
        <span aria-hidden>·</span>
        <time dateTime={it.last_reported_at} suppressHydrationWarning>
          {forumTimeAgo(it.last_reported_at)}
        </time>
        {it.removed && <span className="rounded bg-warning-soft px-1.5 py-0.5 text-warning">{t("mod_already_removed")}</span>}
      </div>

      <div>
        <Link href={href} target="_blank" className="font-medium text-ink hover:text-accent">
          {it.post_title}
        </Link>
        <p className="mt-1 line-clamp-3 text-sm text-muted">{it.excerpt}</p>
      </div>

      <div className="flex flex-wrap items-center gap-x-2 gap-y-1 text-xs">
        <span className="text-muted">{t("mod_author")}</span>
        <span className="font-medium text-ink">
          {it.author.display_name || it.author.handle} <span className="font-normal text-faint">@{it.author.handle}</span>
        </span>
        {it.is_anonymous && <span className="text-faint">({t("forum_posted_anonymously")})</span>}
        {it.author.muted_until && (
          <span className="rounded bg-danger-soft px-1.5 py-0.5 text-danger">
            {t("mod_muted_until_fmt", { params: { date: formatShortDate(it.author.muted_until) } })}
          </span>
        )}
      </div>

      <div className="flex flex-wrap gap-1.5">
        {it.reasons.map((r) => (
          <span key={r} className="rounded-full bg-danger-soft px-2 py-0.5 text-xs text-danger">
            {t(REASON_KEY[r])}
          </span>
        ))}
      </div>
      {it.details.length > 0 && (
        <ul className="space-y-1 border-l-2 border-divider pl-3 text-sm text-muted">
          {it.details.map((d, i) => (
            <li key={i}>&ldquo;{d}&rdquo;</li>
          ))}
        </ul>
      )}

      <div className="flex flex-wrap items-center gap-2 border-t border-divider pt-3">
        <button type="button" onClick={() => onResolve(it, "dismiss")} className={cn(button, "text-muted hover:text-ink")}>
          <Check size={14} />
          {t("mod_dismiss")}
        </button>
        <button type="button" onClick={() => onResolve(it, "remove")} className={cn(button, "text-danger hover:bg-danger-soft")}>
          <Trash2 size={14} />
          {t("mod_remove")}
        </button>
        <span className="ml-auto inline-flex items-center gap-1.5">
          <select
            value={muteDays}
            onChange={(e) => setMuteDays(Number(e.target.value))}
            aria-label={t("mod_mute_days")}
            className="rounded-md border border-divider bg-surface px-2 py-1.5 text-sm"
          >
            {MUTE_DAYS.map((d) => (
              <option key={d} value={d}>
                {daysLabel(d)}
              </option>
            ))}
          </select>
          <button
            type="button"
            onClick={() => onResolve(it, "remove", muteDays)}
            className={cn(button, "border-danger bg-danger text-white hover:opacity-90")}
          >
            <Ban size={14} />
            {t("mod_remove_and_mute")}
          </button>
        </span>
      </div>
    </article>
  );
}

function MuteList() {
  const [mutes, setMutes] = useState<ForumMute[] | null>(null);
  const [handle, setHandle] = useState("");
  const [days, setDays] = useState(MUTE_DAYS[1]);
  const [reason, setReason] = useState("");
  const [busy, setBusy] = useState(false);

  const load = useCallback(async () => {
    try {
      setMutes((await forumAdminApi.mutes()).mutes);
    } catch (e) {
      toast.error(forumErrorMessage(e));
      setMutes([]);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  async function mute() {
    if (!handle.trim() || busy) return;
    setBusy(true);
    try {
      await forumAdminApi.mute(handle.trim(), days, reason.trim());
      setHandle("");
      setReason("");
      toast.success(t("mod_muted_fmt", { params: { days: daysLabel(days) } }));
      await load();
    } catch (e) {
      toast.error(e instanceof Error && e.message ? e.message : forumErrorMessage(e));
    } finally {
      setBusy(false);
    }
  }

  async function unmute(m: ForumMute) {
    try {
      await forumAdminApi.unmute(m.user_id);
      setMutes((prev) => prev?.filter((x) => x.user_id !== m.user_id) ?? null);
      toast.success(t("mod_unmuted"));
    } catch (e) {
      toast.error(forumErrorMessage(e));
    }
  }

  const field = "rounded-md border border-divider bg-surface px-3 py-1.5 text-sm outline-none focus:border-accent";

  return (
    <div className="space-y-4">
      <form
        className="flex flex-wrap items-center gap-2 rounded-xl border border-divider bg-surface p-4"
        onSubmit={(e) => {
          e.preventDefault();
          void mute();
        }}
      >
        <input
          value={handle}
          onChange={(e) => setHandle(e.target.value)}
          placeholder={t("mod_handle_placeholder")}
          aria-label={t("mod_handle_placeholder")}
          className={cn(field, "min-w-[10rem] flex-1")}
        />
        <select value={days} onChange={(e) => setDays(Number(e.target.value))} aria-label={t("mod_mute_days")} className={field}>
          {MUTE_DAYS.map((d) => (
            <option key={d} value={d}>
              {daysLabel(d)}
            </option>
          ))}
        </select>
        <input
          value={reason}
          onChange={(e) => setReason(e.target.value)}
          placeholder={t("mod_reason_placeholder")}
          aria-label={t("mod_reason_placeholder")}
          className={cn(field, "min-w-[10rem] flex-1")}
        />
        <button
          type="submit"
          disabled={!handle.trim() || busy}
          className="rounded-md bg-danger px-3.5 py-1.5 text-sm font-medium text-white disabled:opacity-50"
        >
          {t("mod_mute")}
        </button>
      </form>

      {mutes === null ? (
        <p className="py-8 text-center text-sm text-muted">{t("loading")}</p>
      ) : mutes.length === 0 ? (
        <p className="py-8 text-center text-sm text-muted">{t("mod_no_mutes")}</p>
      ) : (
        <ul className="divide-y divide-divider overflow-hidden rounded-xl border border-divider bg-surface">
          {mutes.map((m) => (
            <li key={m.user_id} className="flex flex-wrap items-center gap-x-3 gap-y-1 px-4 py-3 text-sm">
              <span className="font-medium text-ink">
                {m.display_name || m.handle} <span className="font-normal text-faint">@{m.handle}</span>
              </span>
              <span className="text-muted">
                {t("mod_muted_until_fmt", { params: { date: formatShortDate(m.muted_until) } })}
              </span>
              {m.reason && <span className="text-faint">&ldquo;{m.reason}&rdquo;</span>}
              <button type="button" onClick={() => void unmute(m)} className="ml-auto text-sm text-accent hover:underline">
                {t("mod_unmute")}
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
