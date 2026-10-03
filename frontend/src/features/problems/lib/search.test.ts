import { describe, expect, it } from "vitest";

import { hasFilters, parseFilters, problemsHref, searchQuery } from "./search";

describe("parseFilters", () => {
  it("keeps known values and trims free text", () => {
    expect(
      parseFilters({ category: "security", difficulty: "hard", status: "solved", tag: " Caching ", q: " jwt " }),
    ).toEqual({ category: "security", difficulty: "hard", status: "solved", tag: "Caching", q: "jwt" });
  });

  it("drops unknown enum values and blanks", () => {
    const f = parseFilters({ category: "nope", difficulty: "all", status: "done", tag: " ", q: "" });
    expect(hasFilters(f)).toBe(false);
  });
});

describe("problemsHref", () => {
  it("is the bare page without filters", () => {
    expect(problemsHref({})).toBe("/problems");
  });

  it("encodes tags with spaces and ampersands", () => {
    expect(problemsHref({ tag: "Date & Time", difficulty: "easy" })).toBe(
      "/problems?difficulty=easy&tag=Date+%26+Time",
    );
  });
});

describe("searchQuery", () => {
  it("renames status to user_status and adds paging", () => {
    const qs = new URLSearchParams(searchQuery({ status: "attempted", category: "debugging" }, 50));
    expect(Object.fromEntries(qs)).toEqual({
      category: "debugging",
      user_status: "attempted",
      limit: "50",
      offset: "50",
    });
  });

  it("omits offset on the first page", () => {
    expect(searchQuery({})).toBe("limit=50");
  });
});
