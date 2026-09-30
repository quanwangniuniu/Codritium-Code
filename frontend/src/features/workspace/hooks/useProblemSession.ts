// React hook over the per-user problem-session store.
//
// Usage in problems/[slug]/page.tsx:
//
//   const { session, setSession } = useProblemSession(me?.handle ?? null, slug);
//   session.fileContents[activeTab]                // read
//   setSession(prev => ({ ...prev, fileContents: { ...prev.fileContents, [name]: value } }))
//
// Login / logout (via the /login page + Logout button) changes `handle`
// when the cookie flips; this hook then rebinds to the new handle's state.
// The previous handle's snapshot stays in the store, so signing back in
// restores their prior work.
//
// IMPORTANT: the hook intentionally does NOT touch localStorage in this
// phase. Persistence layer is added in P3 via shared/lib/persisted-store.ts behind
// the same getSession/putSession API; this hook will switch backing
// transparently.

import { useEffect, useState, useCallback } from "react";
import {
  getSession,
  putSession,
  subscribe,
  type ProblemSession,
} from "@/features/workspace/lib/session-store";

export type SetSessionFn = (
  updater: ProblemSession | ((prev: ProblemSession) => ProblemSession),
) => void;

export function useProblemSession(
  handle: string | null,
  slug: string | null,
): { session: ProblemSession | null; setSession: SetSessionFn } {
  // We mirror the store entry into local React state so component reads
  // are synchronous and trigger re-renders on update.
  const [session, setLocalSession] = useState<ProblemSession | null>(() => {
    if (!handle || !slug) return null;
    return getSession(handle, slug);
  });

  // On handle/slug change: load the entry for the new (handle, slug).
  // The previous handle's state already lives in the store from prior
  // setSession calls, so no explicit flush is needed.
  useEffect(() => {
    if (!handle || !slug) {
      setLocalSession(null);
      return;
    }
    setLocalSession(getSession(handle, slug));
  }, [handle, slug]);

  // Listen for external mutations (e.g. another component calls putSession
  // for the same key) and update local state.
  useEffect(() => {
    if (!handle || !slug) return;
    return subscribe((h, s, next) => {
      if (h === handle && s === slug) setLocalSession(next);
    });
  }, [handle, slug]);

  const setSession = useCallback<SetSessionFn>(
    (updater) => {
      if (!handle || !slug) return;
      const current = getSession(handle, slug);
      const next =
        typeof updater === "function"
          ? (updater as (prev: ProblemSession) => ProblemSession)(current)
          : updater;
      putSession(handle, slug, next);
      // setLocalSession will be triggered by the subscribe listener above.
    },
    [handle, slug],
  );

  return { session, setSession };
}
