"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { Bell } from "lucide-react";
import { t } from "@/shared/i18n";
import { useLocale } from "@/shared/i18n/client";
import { notificationsApi, type AppNotification } from "@/features/notifications/api";
import { NotificationItem } from "@/features/notifications/components/NotificationItem";

const POLL_MS = 60_000;
const PREVIEW = 8;

// The nav bell: an unread badge kept fresh by polling (and on tab focus or
// navigation), and a dropdown of the latest notifications.
export function NotificationBell() {
  useLocale();
  const pathname = usePathname();
  const [count, setCount] = useState(0);
  const [open, setOpen] = useState(false);
  const [items, setItems] = useState<AppNotification[] | null>(null);
  const ref = useRef<HTMLDivElement>(null);

  const refreshCount = useCallback(async () => {
    try {
      setCount((await notificationsApi.unreadCount()).count);
    } catch {
      // Signed out or offline: keep the last known count.
    }
  }, []);

  useEffect(() => {
    void refreshCount();
    const timer = setInterval(() => {
      if (document.visibilityState === "visible") void refreshCount();
    }, POLL_MS);
    const onFocus = () => void refreshCount();
    window.addEventListener("focus", onFocus);
    return () => {
      clearInterval(timer);
      window.removeEventListener("focus", onFocus);
    };
  }, [refreshCount, pathname]);

  useEffect(() => {
    if (!open) return;
    let cancelled = false;
    notificationsApi
      .list({ limit: PREVIEW })
      .then((page) => !cancelled && setItems(page.notifications))
      .catch(() => !cancelled && setItems([]));
    void refreshCount();
    const onDown = (e: MouseEvent) => {
      if (!ref.current?.contains(e.target as Node)) setOpen(false);
    };
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && setOpen(false);
    window.addEventListener("mousedown", onDown);
    window.addEventListener("keydown", onKey);
    return () => {
      cancelled = true;
      window.removeEventListener("mousedown", onDown);
      window.removeEventListener("keydown", onKey);
    };
  }, [open, refreshCount]);

  function opened(n: AppNotification) {
    setOpen(false);
    if (n.read) return;
    setCount((c) => Math.max(0, c - 1));
    setItems((prev) => prev?.map((x) => (x.id === n.id ? { ...x, read: true } : x)) ?? null);
    void notificationsApi.markRead([n.id]).catch(() => {});
  }

  async function markAll() {
    setCount(0);
    setItems((prev) => prev?.map((x) => ({ ...x, read: true })) ?? null);
    try {
      await notificationsApi.markAllRead();
    } catch {
      void refreshCount();
    }
  }

  const label = count > 0 ? `${t("nav_notifications")} (${t("notif_unread_count_fmt", { params: { n: count } })})` : t("nav_notifications");

  return (
    <div ref={ref} className="relative">
      <button
        type="button"
        aria-label={label}
        aria-haspopup="dialog"
        aria-expanded={open}
        onClick={() => setOpen((v) => !v)}
        className="nav-icon-btn relative"
      >
        <Bell size={18} strokeWidth={1.8} />
        {count > 0 && (
          <span className="absolute -right-0.5 -top-0.5 grid h-4 min-w-4 place-items-center rounded-full bg-danger px-1 text-[10px] font-semibold leading-none text-white">
            {count > 99 ? "99+" : count}
          </span>
        )}
      </button>
      {open && (
        <div
          role="dialog"
          aria-label={t("notif_title")}
          className="fixed inset-x-2 top-14 z-40 overflow-hidden rounded-xl border border-divider bg-surface shadow-xl sm:absolute sm:inset-x-auto sm:right-0 sm:top-full sm:mt-2 sm:w-96"
        >
          <div className="flex items-center justify-between border-b border-divider px-4 py-2.5">
            <h2 className="text-sm font-semibold">{t("notif_title")}</h2>
            {count > 0 && (
              <button type="button" onClick={() => void markAll()} className="text-xs text-accent hover:underline">
                {t("notif_mark_all_read")}
              </button>
            )}
          </div>
          <div className="max-h-[60vh] overflow-y-auto">
            {items === null ? (
              <p className="px-4 py-8 text-center text-sm text-muted">{t("loading")}</p>
            ) : items.length === 0 ? (
              <p className="px-4 py-8 text-center text-sm text-muted">{t("notif_empty")}</p>
            ) : (
              <ul className="divide-y divide-divider">
                {items.map((n) => (
                  <li key={n.id}>
                    <NotificationItem n={n} onOpen={opened} compact />
                  </li>
                ))}
              </ul>
            )}
          </div>
          <Link
            href="/notifications"
            onClick={() => setOpen(false)}
            className="block border-t border-divider px-4 py-2.5 text-center text-sm text-accent hover:bg-surface-2"
          >
            {t("notif_view_all")}
          </Link>
        </div>
      )}
    </div>
  );
}
