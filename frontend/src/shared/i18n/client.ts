"use client";

import { useSyncExternalStore } from "react";

import {
  type Locale,
  type LocaleKey,
  getServerSnapshot,
  getSnapshot,
  setLocale,
  subscribe,
} from "@/shared/i18n";

// useLocale subscribes a Client Component to the locale store. The non-hook
// pieces (subscribe / getSnapshot / setLocale) live in ./index.ts so server
// components can still call t() during SSR without crossing the
// Client / Server boundary.
export function useLocale(): { locale: Locale; setLocale: (l: Locale) => void } {
  const locale = useSyncExternalStore(subscribe, getSnapshot, getServerSnapshot);
  return { locale, setLocale };
}

export type { Locale, LocaleKey };
