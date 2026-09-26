// Session-store abstraction — opaque key/value persistence with debounced
// writes and a graceful quota fallback. The only backing currently is
// browser localStorage; future AWS deployment is expected to plug in a
// remote sync layer behind the same getRaw/putRaw/deleteRaw interface
// without touching callers.
//
// Keys are owned by the caller; convention is "codritium_session_{handle}_{slug}".
// Values are arbitrary JSON-serialisable objects.

import { toast } from "@/lib/toast";
import { t as tr } from "@/lib/i18n";

const DEBOUNCE_MS = 500;

// Pending writes keyed by storage key. The latest value wins.
const pendingWrites = new Map<string, { value: unknown; timer: number }>();

// Is localStorage usable? (SSR-safe / Safari private-mode safe check.)
function hasLocalStorage(): boolean {
  if (typeof window === "undefined") return false;
  try {
    const probe = "__codritium_probe__";
    window.localStorage.setItem(probe, "1");
    window.localStorage.removeItem(probe);
    return true;
  } catch {
    return false;
  }
}

let quotaToastShown = false;

function writeOrEvict(key: string, value: unknown): void {
  if (!hasLocalStorage()) return;
  const serialised = JSON.stringify(value);
  try {
    window.localStorage.setItem(key, serialised);
    quotaToastShown = false;
  } catch (e) {
    if (isQuotaError(e)) {
      // Evict the oldest other codritium_session_* key and try once more.
      const evicted = evictOldestCodritiumKey(key);
      if (evicted) {
        try {
          window.localStorage.setItem(key, serialised);
          return;
        } catch {
          // fall through to user-facing toast
        }
      }
      if (!quotaToastShown) {
        toast.warn(tr("session_quota_exceeded_oldest_evicted"));
        quotaToastShown = true;
      }
    } else {
      console.error("session-store write failed:", e);
    }
  }
}

function isQuotaError(e: unknown): boolean {
  if (!(e instanceof DOMException)) return false;
  // Different browsers report different names/codes for the same condition.
  return (
    e.name === "QuotaExceededError" ||
    e.name === "NS_ERROR_DOM_QUOTA_REACHED" ||
    e.code === 22 ||
    e.code === 1014
  );
}

function evictOldestCodritiumKey(skip: string): string | null {
  if (!hasLocalStorage()) return null;
  const ls = window.localStorage;
  let oldestKey: string | null = null;
  // Without timestamps in keys, "oldest" is approximated by enumeration
  // order, which is insertion order in practice for localStorage.
  for (let i = 0; i < ls.length; i++) {
    const k = ls.key(i);
    if (k && k !== skip && k.startsWith("codritium_session_")) {
      oldestKey = k;
      break;
    }
  }
  if (oldestKey) ls.removeItem(oldestKey);
  return oldestKey;
}

// Synchronously read the persisted value, if any.
export function getRaw<T>(key: string): T | null {
  if (!hasLocalStorage()) return null;
  try {
    const raw = window.localStorage.getItem(key);
    if (!raw) return null;
    return JSON.parse(raw) as T;
  } catch (e) {
    console.error("session-store read failed:", key, e);
    return null;
  }
}

// Debounced write — multiple putRaw calls within DEBOUNCE_MS for the same
// key coalesce; only the latest value reaches localStorage.
export function putRaw(key: string, value: unknown): void {
  if (typeof window === "undefined") return;
  const existing = pendingWrites.get(key);
  if (existing) window.clearTimeout(existing.timer);
  const timer = window.setTimeout(() => {
    pendingWrites.delete(key);
    writeOrEvict(key, value);
  }, DEBOUNCE_MS);
  pendingWrites.set(key, { value, timer });
}

// Immediate write — flushes any pending debounced write for the same key
// first. Use when you need persistence to be done before a navigation /
// process exit (e.g. on sign-out).
export function putRawNow(key: string, value: unknown): void {
  if (typeof window === "undefined") return;
  const existing = pendingWrites.get(key);
  if (existing) {
    window.clearTimeout(existing.timer);
    pendingWrites.delete(key);
  }
  writeOrEvict(key, value);
}

export function deleteRaw(key: string): void {
  if (typeof window === "undefined") return;
  const existing = pendingWrites.get(key);
  if (existing) {
    window.clearTimeout(existing.timer);
    pendingWrites.delete(key);
  }
  if (!hasLocalStorage()) return;
  try {
    window.localStorage.removeItem(key);
  } catch (e) {
    console.error("session-store delete failed:", key, e);
  }
}

// Useful for tests / hard reset; not used by normal flow.
export function listKeys(prefix: string): string[] {
  if (!hasLocalStorage()) return [];
  const ls = window.localStorage;
  const out: string[] = [];
  for (let i = 0; i < ls.length; i++) {
    const k = ls.key(i);
    if (k && k.startsWith(prefix)) out.push(k);
  }
  return out;
}
