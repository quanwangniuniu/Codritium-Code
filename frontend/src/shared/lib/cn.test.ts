import { describe, expect, it } from "vitest";
import { cn } from "@/shared/lib/cn";

describe("cn", () => {
  it("joins truthy classes and lets later tailwind classes win", () => {
    expect(cn("p-2", false && "hidden", "p-4")).toBe("p-4");
  });
});
