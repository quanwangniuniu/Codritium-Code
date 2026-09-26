// Per-user problem session store — keeps in-progress state separate per
// (handle, slug) so the wrong user never sees / loses someone else's work.
//
// Storage layers:
//   1. In-memory cache (this module's `store` map) — fast reads, listener
//      notifications for React.
//   2. localStorage via lib/session-store.ts — durability across hard
//      refresh / closed-tab. Reads are lazy (cache miss hydrates from
//      localStorage); writes go through both cache and the debounced
//      localStorage writer.
//
// Key naming: `codritium_session_{handle}_{slug}` (caller never touches keys
// directly; everything flows through getSession / putSession / deleteSession).

import type { ChatMessage } from "@/components/ChatPanel";
import type { OpenTab } from "@/lib/types";
import type { EditorPaneState } from "@/components/ide/EditorAreaSplit";
import { deleteRaw, getRaw, putRaw } from "@/lib/session-store";

export interface ProblemSession {
  // user's edits keyed by filename
  fileContents: Record<string, string>;
  // pristine starter snapshot for dirty-detection
  originalContents: Record<string, string>;
  // legacy editor tabs (mirrored from editorPanes[0] for restore-on-reload
  // and intro-page peek; multi-pane state lives in editorPanes).
  openTabs: OpenTab[];
  activeTab: string | null;
  // Per-pane editor state. Missing on legacy persisted sessions; the
  // workspace migrates by deriving one pane from openTabs/activeTab.
  editorPanes?: EditorPaneState[];
  // chat history
  messages: ChatMessage[];
  // timer start (ms epoch); preserved across sign-in cycles so a returning
  // user sees their original elapsed counter resume
  startedAt: number;
  // submission id created by chat (auto-create on first message), reused on submit
  submissionId: string | null;
  // ms epoch — refreshed on every putSession. Drives the "Last edited X ago"
  // label on the problem intro page.
  lastUpdatedAt?: number;
  // True when the session was just hydrated from localStorage and has real
  // user work (not just freshly-seeded starter). Lets the page show a
  // one-shot "restored from previous session" toast and then clear the flag.
  // Always cleared before persisting.
  restoredFromStorage?: boolean;
}

function emptySession(): ProblemSession {
  return {
    fileContents: {},
    originalContents: {},
    openTabs: [],
    activeTab: null,
    messages: [],
    startedAt: Date.now(),
    submissionId: null,
  };
}

function storageKey(handle: string, slug: string): string {
  return `codritium_session_${handle}_${slug}`;
}

// Did this restored session have any actual user work (vs. just seeded starter)?
function hasRealWork(s: ProblemSession): boolean {
  if (s.messages.length > 0) return true;
  for (const [name, content] of Object.entries(s.fileContents)) {
    if (content !== s.originalContents[name]) return true;
  }
  return false;
}

// Outer key: user handle. Inner key: problem slug.
type Store = Map<string, Map<string, ProblemSession>>;
const store: Store = new Map();

type Listener = (handle: string, slug: string, session: ProblemSession) => void;
const listeners: Set<Listener> = new Set();

function emit(handle: string, slug: string, session: ProblemSession): void {
  for (const l of listeners) l(handle, slug, session);
}

function ensureUserMap(handle: string): Map<string, ProblemSession> {
  let userMap = store.get(handle);
  if (!userMap) {
    userMap = new Map();
    store.set(handle, userMap);
  }
  return userMap;
}

// Lazily hydrate from localStorage on cache miss.
export function getSession(handle: string, slug: string): ProblemSession {
  const userMap = ensureUserMap(handle);
  const cached = userMap.get(slug);
  if (cached) return cached;

  const stored = getRaw<ProblemSession>(storageKey(handle, slug));
  if (stored) {
    // Tag for one-time restore toast on the consuming page.
    stored.restoredFromStorage = hasRealWork(stored);
    userMap.set(slug, stored);
    return stored;
  }

  const fresh = emptySession();
  userMap.set(slug, fresh);
  return fresh;
}

export function putSession(handle: string, slug: string, session: ProblemSession): void {
  const userMap = ensureUserMap(handle);
  const stamped: ProblemSession = { ...session, lastUpdatedAt: Date.now() };
  userMap.set(slug, stamped);
  emit(handle, slug, stamped);
  // Don't persist the transient `restoredFromStorage` flag.
  const persistable: ProblemSession = { ...stamped };
  delete persistable.restoredFromStorage;
  putRaw(storageKey(handle, slug), persistable);
}

// Peek at session metadata WITHOUT mutating the cache or triggering the
// "restoredFromStorage" tag. Used by the intro page to decide whether to
// show the Resume widget.
export function peekSessionMeta(
  handle: string,
  slug: string,
): { hasRealWork: boolean; lastUpdatedAt: number | null } {
  const userMap = store.get(handle);
  const cached = userMap?.get(slug);
  const s = cached ?? getRaw<ProblemSession>(storageKey(handle, slug));
  if (!s) return { hasRealWork: false, lastUpdatedAt: null };
  return {
    hasRealWork: hasRealWork(s),
    lastUpdatedAt: s.lastUpdatedAt ?? s.startedAt ?? null,
  };
}

export function deleteSession(handle: string, slug: string): void {
  const userMap = store.get(handle);
  if (userMap) userMap.delete(slug);
  deleteRaw(storageKey(handle, slug));
}

// Clears the `restoredFromStorage` flag after the consuming page has shown
// its restore toast. Does NOT trigger listeners — flag-only mutation.
export function clearRestoredFlag(handle: string, slug: string): void {
  const userMap = store.get(handle);
  if (!userMap) return;
  const session = userMap.get(slug);
  if (!session || !session.restoredFromStorage) return;
  delete session.restoredFromStorage;
}

// Has the session diverged from its original starter state?
export function isDirty(session: ProblemSession): boolean {
  for (const [name, content] of Object.entries(session.fileContents)) {
    if (content !== session.originalContents[name]) return true;
  }
  return session.messages.length > 0;
}

// Has the current (handle, slug) session been touched at all (file edited or
// chat sent)? Useful for "is there anything to save?" checks before user
// switch.
export function hasUnsavedWork(handle: string | null, slug: string | null): boolean {
  if (!handle || !slug) return false;
  const userMap = store.get(handle);
  if (!userMap) return false;
  const session = userMap.get(slug);
  if (!session) return false;
  return isDirty(session);
}

export function subscribe(listener: Listener): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}
