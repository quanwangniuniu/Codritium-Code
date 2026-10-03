import { describe, expect, it } from "vitest";

import { formatBirthday, isGeneratedHandle, linkLabel } from "./fields";

describe("isGeneratedHandle", () => {
  it("recognises minted handles only", () => {
    expect(isGeneratedHandle("u_0ebcfb4594")).toBe(true);
    expect(isGeneratedHandle("john")).toBe(false);
    expect(isGeneratedHandle("u_john")).toBe(false);
    expect(isGeneratedHandle("u_0ebcfb45941")).toBe(false);
  });
});

describe("linkLabel", () => {
  it("drops scheme, www and trailing slash", () => {
    expect(linkLabel("https://www.github.com/ada/")).toBe("github.com/ada");
    expect(linkLabel("http://example.com")).toBe("example.com");
  });
});

describe("formatBirthday", () => {
  it("formats a calendar date without timezone drift", () => {
    expect(formatBirthday("1990-05-17")).toBe("May 17, 1990");
    expect(formatBirthday("2000-01-01")).toBe("January 1, 2000");
  });

  it("returns unparseable input unchanged", () => {
    expect(formatBirthday("")).toBe("");
  });
});
