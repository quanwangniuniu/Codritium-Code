// Codritium i18n core.
//
// Usage:
//   import { t } from "@/shared/i18n";
//   <button>{t("submit")}</button>
//   toast.error(t("submit_failed"));
//   t("prof_member_since", { params: { date } })   // "{date}" interpolation
//
// Rules:
//   - All user-facing strings go through t(). No bare literals like
//     <button>Submit</button>.
//   - Strings live next to the feature that renders them
//     (features/<f>/i18n.ts, or shared/i18n/common.ts for shared ones) and
//     are merged in ./dictionaries.ts. LocaleKey is derived from that merge.
//   - English is the only locale today. To add one: extend LOCALES, add a
//     Partial<Record<LocaleKey, string>> per feature and register it in
//     DICTIONARIES below; missing keys fall back to English.

import { en, type LocaleKey } from "./dictionaries";

export const LOCALES = ["en"] as const;
export type Locale = (typeof LOCALES)[number];
export const DEFAULT_LOCALE: Locale = "en";

const DICTIONARIES: Record<Locale, Partial<Record<LocaleKey, string>>> = { en };

// The locale lives in a module-level external store so that:
//   - non-component callers (toast, session-store, ad-hoc helpers) can stay
//     on the synchronous t() API without touching React;
//   - the client-side `useLocale` hook (./client.ts) subscribes via
//     useSyncExternalStore and re-renders on locale change.
// The store helpers are kept in this server-safe module so server components
// can still call t() during SSR without dragging React hooks across the
// Client / Server boundary.
let currentLocale: Locale = DEFAULT_LOCALE;
const subscribers = new Set<() => void>();
const STORAGE_KEY = "codritium-locale";

export function subscribe(fn: () => void): () => void {
  subscribers.add(fn);
  return () => {
    subscribers.delete(fn);
  };
}

export function getSnapshot(): Locale {
  return currentLocale;
}

export function getServerSnapshot(): Locale {
  // SSR always renders the default locale so the first client paint matches
  // the server-rendered HTML. A saved preference is applied after hydration
  // via loadLocaleFromStorage().
  return DEFAULT_LOCALE;
}

function isLocale(v: unknown): v is Locale {
  return typeof v === "string" && (LOCALES as readonly string[]).includes(v);
}

export function setLocale(locale: Locale): void {
  if (currentLocale === locale) return;
  currentLocale = locale;
  if (typeof window !== "undefined") {
    try {
      window.localStorage.setItem(STORAGE_KEY, locale);
    } catch {
      // localStorage may be unavailable (private mode, quota); the runtime
      // locale still updates so the UI flips this session.
    }
  }
  subscribers.forEach((fn) => fn());
}

// Hydration helper for a future LocaleHydrator client component.
export function loadLocaleFromStorage(): void {
  if (typeof window === "undefined") return;
  try {
    const saved = window.localStorage.getItem(STORAGE_KEY);
    if (isLocale(saved) && saved !== currentLocale) setLocale(saved);
  } catch {
    // ignore — keep current locale on read failure
  }
}

// Lightweight {param} interpolation. Keeps t() synchronous and avoids
// pulling in a full ICU runtime. Falls back to English, then to the key.
export function t(
  key: LocaleKey,
  opts?: { locale?: Locale; params?: Record<string, string | number> },
): string {
  const locale = opts?.locale ?? currentLocale;
  const raw = DICTIONARIES[locale]?.[key];
  const value = raw && raw.length > 0 ? raw : en[key] ?? key;
  const params = opts?.params;
  if (!params) return value;
  return Object.entries(params).reduce(
    (out, [k, v]) => out.replace(new RegExp(`\\{${k}\\}`, "g"), String(v)),
    value,
  );
}

export type { LocaleKey };
