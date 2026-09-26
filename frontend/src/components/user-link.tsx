import Link from "next/link";
import type { User } from "@/lib/types";

export function UserLink({ user }: { user: User }) {
  return (
    <Link
      href={`/u/${encodeURIComponent(user.github_handle)}`}
      className="inline-flex items-center gap-1.5 text-sm hover:text-ink"
    >
      <span className="grid h-5 w-5 place-items-center rounded-full bg-surface-2 text-[10px] font-medium tabular-nums">
        {user.display_name.slice(0, 1)}
      </span>
      <span className="text-muted hover:text-ink">{user.display_name}</span>
    </Link>
  );
}
