// In-app notifications: types, browser API calls, and how each kind reads.

import { t } from "@/shared/i18n";
import { apiRequest } from "@/shared/api/client";

export type NotificationKind = "post_reply" | "comment_reply" | "mention" | "post_comment" | "post_milestone";

export interface NotificationActor {
  id: string;
  handle: string;
  display_name: string;
  avatar_url: string;
  avatar_color: string;
}

export interface AppNotification {
  id: string;
  kind: NotificationKind;
  // null for system events (milestones) and anonymous actors.
  actor: NotificationActor | null;
  post_id: string;
  post_title: string;
  comment_id: string | null;
  excerpt: string;
  milestone: number | null;
  read: boolean;
  created_at: string;
}

export interface NotificationPage {
  notifications: AppNotification[];
  has_more: boolean;
}

export const notificationsApi = {
  list: (opts: { offset?: number; limit?: number; unread?: boolean } = {}) => {
    const params = new URLSearchParams({ limit: String(opts.limit ?? 20) });
    if (opts.offset) params.set("offset", String(opts.offset));
    if (opts.unread) params.set("unread", "1");
    return apiRequest<NotificationPage>(`/api/notifications?${params}`);
  },
  unreadCount: () => apiRequest<{ count: number }>("/api/notifications/unread-count"),
  markRead: (ids: string[]) => apiRequest<void>("/api/notifications/read", { method: "POST", json: { ids } }),
  markAllRead: () => apiRequest<void>("/api/notifications/read", { method: "POST", json: { all: true } }),
};

// Where clicking a notification goes: the comment if there is one.
export function notificationHref(n: AppNotification): string {
  return n.comment_id ? `/forums/${n.post_id}#comment-${n.comment_id}` : `/forums/${n.post_id}`;
}

// The sentence describing what happened, e.g. "Ann replied to your comment".
export function notificationText(n: AppNotification): string {
  const name = n.actor ? n.actor.display_name || n.actor.handle : t("notif_someone");
  switch (n.kind) {
    case "post_reply":
      return t("notif_post_reply_fmt", { params: { name } });
    case "comment_reply":
      return t("notif_comment_reply_fmt", { params: { name } });
    case "mention":
      return t("notif_mention_fmt", { params: { name } });
    case "post_comment":
      return t("notif_post_comment_fmt", { params: { name } });
    case "post_milestone":
      return t("notif_milestone_fmt", { params: { n: n.milestone ?? 0 } });
  }
}
