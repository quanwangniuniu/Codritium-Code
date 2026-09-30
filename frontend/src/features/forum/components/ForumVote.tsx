"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { ArrowBigDown, ArrowBigUp } from "lucide-react";
import { t } from "@/shared/i18n";
import { useLocale } from "@/shared/i18n/client";
import { toast } from "@/shared/lib/toast";
import { cn } from "@/shared/lib/cn";
import { forumApi, forumErrorMessage } from "@/features/forum/api";
import { compactCount } from "@/shared/format";

interface ForumVoteProps {
  kind: "post" | "comment";
  id: string;
  score: number;
  myVote: -1 | 0 | 1;
  signedIn: boolean;
  size?: "sm" | "md";
}

// Up/down vote pill. Clicking the active arrow again clears the vote.
// The score updates optimistically and rolls back if the request fails.
export function ForumVote({ kind, id, score, myVote, signedIn, size = "md" }: ForumVoteProps) {
  useLocale();
  const router = useRouter();
  const [state, setState] = useState({ score, myVote });
  const [busy, setBusy] = useState(false);

  async function vote(direction: 1 | -1) {
    if (!signedIn) {
      router.push(`/login?next=${encodeURIComponent(window.location.pathname)}`);
      return;
    }
    if (busy) return;
    const next: -1 | 0 | 1 = state.myVote === direction ? 0 : direction;
    const prev = state;
    setState({ score: state.score - state.myVote + next, myVote: next });
    setBusy(true);
    try {
      const res = await (kind === "post" ? forumApi.votePost(id, next) : forumApi.voteComment(id, next));
      setState({ score: res.score, myVote: res.my_vote });
    } catch (e) {
      setState(prev);
      toast.error(forumErrorMessage(e));
    } finally {
      setBusy(false);
    }
  }

  const icon = size === "sm" ? 16 : 20;
  const btn = "grid place-items-center rounded-full transition-colors hover:bg-surface-2 disabled:opacity-60";
  return (
    <div
      className={cn(
        "inline-flex items-center rounded-full border border-divider",
        size === "sm" ? "gap-0.5 px-0.5 text-xs" : "gap-1 px-1 text-sm",
      )}
    >
      <button
        type="button"
        onClick={() => vote(1)}
        disabled={busy}
        aria-label={t("forum_upvote")}
        aria-pressed={state.myVote === 1}
        className={cn(btn, size === "sm" ? "h-6 w-6" : "h-8 w-8", state.myVote === 1 ? "text-success" : "text-muted")}
      >
        <ArrowBigUp size={icon} fill={state.myVote === 1 ? "currentColor" : "none"} />
      </button>
      <span className={cn("min-w-[1.5rem] text-center font-medium tabular-nums", state.myVote !== 0 ? "text-ink" : "text-muted")}>
        {compactCount(state.score)}
      </span>
      <button
        type="button"
        onClick={() => vote(-1)}
        disabled={busy}
        aria-label={t("forum_downvote")}
        aria-pressed={state.myVote === -1}
        className={cn(btn, size === "sm" ? "h-6 w-6" : "h-8 w-8", state.myVote === -1 ? "text-danger" : "text-muted")}
      >
        <ArrowBigDown size={icon} fill={state.myVote === -1 ? "currentColor" : "none"} />
      </button>
    </div>
  );
}
