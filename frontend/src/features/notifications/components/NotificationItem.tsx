"use client";

import Link from "next/link";
import { ArrowBigUp, UserRound } from "lucide-react";
import { UserAvatar } from "@/shared/avatar/UserAvatar";
import { cn } from "@/shared/lib/cn";
import { forumTimeAgo } from "@/features/forum/api";
import { notificationHref, notificationText, type AppNotification } from "@/features/notifications/api";

// One notification row: who, what, which post, a snippet, and when. Unread
// rows carry a dot and a tint.
export function NotificationItem({
  n,
  onOpen,
  compact,
}: {
  n: AppNotification;
  onOpen: (n: AppNotification) => void;
  compact?: boolean;
}) {
  return (
    <Link
      href={notificationHref(n)}
      onClick={() => onOpen(n)}
      className={cn(
        "flex gap-3 px-4 py-3 transition-colors hover:bg-surface-2",
        !n.read && "bg-accent-soft/40",
      )}
    >
      <NotificationIcon n={n} />
      <div className="min-w-0 flex-1 text-sm">
        <p className="text-ink">
          {notificationText(n)}
          <span className="text-muted"> · </span>
          <span className="font-medium">{n.post_title}</span>
        </p>
        {n.excerpt && (
          <p className={cn("mt-0.5 text-muted", compact ? "line-clamp-1" : "line-clamp-2")}>{n.excerpt}</p>
        )}
        <time dateTime={n.created_at} className="mt-0.5 block text-xs text-faint" suppressHydrationWarning>
          {forumTimeAgo(n.created_at)}
        </time>
      </div>
      {!n.read && <span className="mt-1.5 size-2 shrink-0 rounded-full bg-accent" aria-hidden />}
    </Link>
  );
}

function NotificationIcon({ n }: { n: AppNotification }) {
  if (n.kind === "post_milestone") {
    return (
      <span className="grid size-8 shrink-0 place-items-center rounded-full bg-success-soft text-success" aria-hidden>
        <ArrowBigUp size={18} />
      </span>
    );
  }
  if (!n.actor) {
    return (
      <span className="grid size-8 shrink-0 place-items-center rounded-full bg-surface-2 text-faint" aria-hidden>
        <UserRound size={16} />
      </span>
    );
  }
  return <UserAvatar url={n.actor.avatar_url} seed={n.actor.handle} size={32} />;
}
