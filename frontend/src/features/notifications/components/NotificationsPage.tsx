"use client";

import { useCallback, useEffect, useState } from "react";
import { t } from "@/shared/i18n";
import { useLocale } from "@/shared/i18n/client";
import { toast } from "@/shared/lib/toast";
import { cn } from "@/shared/lib/cn";
import { notificationsApi, type AppNotification } from "@/features/notifications/api";
import { NotificationItem } from "@/features/notifications/components/NotificationItem";

const PAGE = 20;

// The full inbox at /notifications: All / Unread, load more, mark all read.
export function NotificationsPage() {
  useLocale();
  const [unreadOnly, setUnreadOnly] = useState(false);
  const [items, setItems] = useState<AppNotification[]>([]);
  const [hasMore, setHasMore] = useState(false);
  const [loading, setLoading] = useState(true);

  const load = useCallback(async (unread: boolean, offset: number) => {
    setLoading(true);
    try {
      const page = await notificationsApi.list({ unread, offset, limit: PAGE });
      setItems((prev) => (offset === 0 ? page.notifications : [...prev, ...page.notifications]));
      setHasMore(page.has_more);
    } catch {
      toast.error(t("forum_err_generic"));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load(unreadOnly, 0);
  }, [load, unreadOnly]);

  function opened(n: AppNotification) {
    if (n.read) return;
    setItems((prev) => prev.map((x) => (x.id === n.id ? { ...x, read: true } : x)));
    void notificationsApi.markRead([n.id]).catch(() => {});
  }

  async function markAll() {
    try {
      await notificationsApi.markAllRead();
      if (unreadOnly) setItems([]);
      else setItems((prev) => prev.map((x) => ({ ...x, read: true })));
      setHasMore(false);
    } catch {
      toast.error(t("forum_err_generic"));
    }
  }

  const anyUnread = items.some((n) => !n.read);

  return (
    <div className="mx-auto max-w-2xl space-y-4 px-4 py-6 sm:px-6 lg:py-8">
      <div className="flex flex-wrap items-center gap-3">
        <h1 className="text-xl font-semibold">{t("notif_title")}</h1>
        <div className="ml-auto flex items-center gap-1 text-sm" role="group" aria-label={t("notif_title")}>
          {[false, true].map((u) => (
            <button
              key={String(u)}
              type="button"
              aria-pressed={unreadOnly === u}
              onClick={() => setUnreadOnly(u)}
              className={cn(
                "rounded-md px-2.5 py-1 transition-colors",
                unreadOnly === u ? "bg-surface-2 font-medium text-ink" : "text-muted hover:text-ink",
              )}
            >
              {u ? t("notif_unread") : t("notif_all")}
            </button>
          ))}
        </div>
        {anyUnread && (
          <button type="button" onClick={() => void markAll()} className="text-sm text-accent hover:underline">
            {t("notif_mark_all_read")}
          </button>
        )}
      </div>

      <div className="overflow-hidden rounded-xl border border-divider bg-surface">
        {loading && items.length === 0 ? (
          <p className="px-4 py-12 text-center text-sm text-muted">{t("loading")}</p>
        ) : items.length === 0 ? (
          <p className="px-4 py-12 text-center text-sm text-muted">
            {unreadOnly ? t("notif_empty_unread") : t("notif_empty")}
          </p>
        ) : (
          <ul className="divide-y divide-divider">
            {items.map((n) => (
              <li key={n.id}>
                <NotificationItem n={n} onOpen={opened} />
              </li>
            ))}
          </ul>
        )}
      </div>

      {hasMore && (
        <div className="flex justify-center">
          <button
            type="button"
            disabled={loading}
            onClick={() => void load(unreadOnly, items.length)}
            className="rounded-md border border-divider px-4 py-2 text-sm text-muted hover:text-ink disabled:opacity-60"
          >
            {loading ? t("loading") : t("notif_load_more")}
          </button>
        </div>
      )}
    </div>
  );
}
