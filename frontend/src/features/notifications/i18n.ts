// Notification strings.
// English is the source of truth; LocaleKey is derived from the merged
// dictionaries in shared/i18n/dictionaries.ts.
export const notificationsEn = {
  notif_all: "All",
  notif_comment_reply_fmt: "{name} replied to your comment",
  notif_empty: "You're all caught up.",
  notif_empty_unread: "No unread notifications.",
  notif_load_more: "Load more",
  notif_mark_all_read: "Mark all as read",
  notif_mention_fmt: "{name} mentioned you",
  notif_milestone_fmt: "Your post reached {n} upvotes",
  notif_post_comment_fmt: "{name} commented on a post you follow",
  notif_post_reply_fmt: "{name} commented on your post",
  notif_someone: "Someone",
  notif_title: "Notifications",
  notif_unread: "Unread",
  notif_unread_count_fmt: "{n} unread",
  notif_view_all: "View all notifications",
} satisfies Record<string, string>;
