// Small pure helpers for profile fields, shared by the profile page and the
// edit page.

// Sign-up mints handles like "u_3fa91c07be" (see createPasswordUser and the
// OAuth path in the backend). Those are ids, not names a person chose, so
// the UI hides them instead of presenting "@u_3fa91c07be" as a username.
export function isGeneratedHandle(handle: string): boolean {
  return /^u_[0-9a-f]{10}$/.test(handle);
}

// A stored profile link without the scheme and "www.", e.g.
// "https://www.github.com/ada/" -> "github.com/ada".
export function linkLabel(url: string): string {
  return url.replace(/^https?:\/\/(www\.)?/, "").replace(/\/$/, "");
}

// "1990-05-17" -> "May 17, 1990". Parsed as a calendar date, not an
// instant, so the viewer's timezone can never shift it by a day.
export function formatBirthday(isoDate: string): string {
  const [y, m, d] = isoDate.split("-").map(Number);
  if (!y || !m || !d) return isoDate;
  return new Date(Date.UTC(y, m - 1, d)).toLocaleDateString("en-US", {
    year: "numeric",
    month: "long",
    day: "numeric",
    timeZone: "UTC",
  });
}
