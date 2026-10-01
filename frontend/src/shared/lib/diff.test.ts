import { describe, expect, it } from "vitest";
import { diffStats, unifiedDiff } from "@/shared/lib/diff";

describe("unifiedDiff", () => {
  it("returns only equal lines for identical input", () => {
    const d = unifiedDiff("a\nb", "a\nb");
    expect(d).toEqual([
      { op: "=", text: "a" },
      { op: "=", text: "b" },
    ]);
  });

  it("marks replaced, added and removed lines in order", () => {
    const d = unifiedDiff("a\nb\nc", "a\nB\nc\nd");
    expect(d.map((l) => `${l.op}${l.text}`)).toEqual(["=a", "+B", "-b", "=c", "+d"]);
  });

  it("handles empty sides", () => {
    expect(unifiedDiff("", "x").map((l) => l.op).sort()).toEqual(["+", "-"]);
    expect(unifiedDiff("x\ny", "").filter((l) => l.op === "-")).toHaveLength(2);
  });
});

describe("diffStats", () => {
  it("counts each op", () => {
    expect(diffStats(unifiedDiff("a\nb\nc", "a\nB\nc\nd"))).toEqual({
      added: 2,
      removed: 1,
      unchanged: 2,
    });
  });
});
