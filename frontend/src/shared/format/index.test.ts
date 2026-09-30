import { describe, expect, it } from "vitest";
import {
  compactCount,
  formatDate,
  formatDuration,
  formatMonthYear,
  formatRelativeTime,
  formatShortDate,
  initialOf,
} from "@/shared/format";

const NOW = Date.UTC(2026, 8, 30, 12, 0, 0);
const ago = (secs: number) => NOW - secs * 1000;

describe("formatRelativeTime", () => {
  it.each([
    [10, "just now"],
    [90, "1 minute ago"],
    [150, "2 minutes ago"],
    [60 * 30, "30 minutes ago"],
    [3600, "1 hour ago"],
    [3600 * 5, "5 hours ago"],
    [86_400, "yesterday"],
    [86_400 * 3, "3 days ago"],
    [604_800, "last week"],
    [2_592_000 * 2, "2 months ago"],
    [31_536_000 * 2, "2 years ago"],
  ])("%is ago -> %s", (secs, expected) => {
    expect(formatRelativeTime(ago(secs), NOW)).toBe(expected);
  });

  it("accepts ISO strings and clamps the future", () => {
    expect(formatRelativeTime(new Date(ago(7200)).toISOString(), NOW)).toBe("2 hours ago");
    expect(formatRelativeTime(NOW + 60_000, NOW)).toBe("just now");
  });
});

describe("date formatting", () => {
  const d = new Date(2026, 8, 30, 15, 4, 5);
  it("uses the en locale", () => {
    expect(formatMonthYear(d)).toBe("Sep 2026");
    expect(formatShortDate(d)).toBe("Sep 30, 2026");
    expect(formatDate(d)).toBe("9/30/2026");
  });
});

describe("compactCount", () => {
  it.each([
    [0, "0"],
    [999, "999"],
    [1000, "1K"],
    [1234, "1.2K"],
    [25_100, "25.1K"],
    [1_500_000, "1.5M"],
  ])("%i -> %s", (n, expected) => {
    expect(compactCount(n)).toBe(expected);
  });
});

describe("formatDuration", () => {
  it("pads hours, minutes and seconds", () => {
    expect(formatDuration(0)).toBe("00:00:00");
    expect(formatDuration(3723)).toBe("01:02:03");
    expect(formatDuration(-5)).toBe("00:00:00");
  });
});

describe("initialOf", () => {
  it("returns the first letter of the first non-empty name", () => {
    expect(initialOf("ada")).toBe("A");
    expect(initialOf("", "  bob")).toBe("B");
    expect(initialOf(undefined, null, "")).toBe("?");
  });
});
