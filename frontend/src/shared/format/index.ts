// Date / time / count formatting. Everything renders in the "en" locale so
// server-rendered HTML and the client's first paint agree regardless of the
// browser's locale (a bare toLocaleString() would cause hydration drift).

export const FORMAT_LOCALE = "en";

type DateInput = string | number | Date;

const toDate = (d: DateInput) => (d instanceof Date ? d : new Date(d));

// "Sep 30, 2026, 10:04:05 PM"
export function formatDateTime(d: DateInput): string {
  return toDate(d).toLocaleString(FORMAT_LOCALE);
}

// "9/30/2026" by default; pass Intl options for other shapes.
export function formatDate(d: DateInput, opts?: Intl.DateTimeFormatOptions): string {
  return toDate(d).toLocaleDateString(FORMAT_LOCALE, opts);
}

// "Sep 2026"
export function formatMonthYear(d: DateInput): string {
  return formatDate(d, { month: "short", year: "numeric" });
}

// "Sep 30, 2026"
export function formatShortDate(d: DateInput): string {
  return formatDate(d, { month: "short", day: "numeric", year: "numeric" });
}

const rtf = new Intl.RelativeTimeFormat(FORMAT_LOCALE, { numeric: "auto" });

const STEPS: [Intl.RelativeTimeFormatUnit, number][] = [
  ["year", 31_536_000],
  ["month", 2_592_000],
  ["week", 604_800],
  ["day", 86_400],
  ["hour", 3_600],
  ["minute", 60],
];

// Long-form relative time: "just now", "5 minutes ago", "yesterday",
// "last week", "3 months ago". Future dates clamp to "just now".
export function formatRelativeTime(d: DateInput, now: number = Date.now()): string {
  const diff = Math.min(0, (toDate(d).getTime() - now) / 1000);
  if (Math.abs(diff) < 45) return "just now";
  for (const [unit, secs] of STEPS) {
    if (Math.abs(diff) >= secs) return rtf.format(Math.round(diff / secs), unit);
  }
  return rtf.format(-1, "minute");
}

// 1234 -> "1.2K", 25100 -> "25.1K", like LeetCode's view counts.
export function compactCount(n: number): string {
  if (n < 1000) return String(n);
  if (n < 1_000_000) return `${(n / 1000).toFixed(1).replace(/\.0$/, "")}K`;
  return `${(n / 1_000_000).toFixed(1).replace(/\.0$/, "")}M`;
}

// "01:02:03" from a number of seconds.
export function formatDuration(totalSeconds: number): string {
  const s = Math.max(0, Math.floor(totalSeconds));
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${pad(Math.floor(s / 3600))}:${pad(Math.floor((s % 3600) / 60))}:${pad(s % 60)}`;
}

// First letter of the first non-empty name, upper-cased; "?" when none.
export function initialOf(...names: (string | null | undefined)[]): string {
  for (const n of names) {
    const c = n?.trim().charAt(0);
    if (c) return c.toUpperCase();
  }
  return "?";
}
