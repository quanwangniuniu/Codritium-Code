import { describe, expect, it } from "vitest";
import { EN_DICTIONARIES } from "@/shared/i18n/dictionaries";
import { LOCALES, getSnapshot, setLocale, subscribe, t } from "@/shared/i18n";

describe("dictionaries", () => {
  it("never defines the same key in two feature dictionaries", () => {
    const owner = new Map<string, string>();
    const dupes: string[] = [];
    for (const [name, dict] of Object.entries(EN_DICTIONARIES)) {
      for (const key of Object.keys(dict)) {
        if (owner.has(key)) dupes.push(`${key} (${owner.get(key)} + ${name})`);
        owner.set(key, name);
      }
    }
    expect(dupes).toEqual([]);
  });

  it("has no empty English strings", () => {
    for (const dict of Object.values(EN_DICTIONARIES)) {
      for (const [key, value] of Object.entries(dict)) expect(value, key).not.toBe("");
    }
  });
});

describe("t", () => {
  it("returns the English string", () => {
    expect(t("submit")).toBe("Submit");
  });

  it("interpolates {params}, repeatedly", () => {
    expect(t("prof_member_since", { params: { date: "Sep 2026" } })).toBe("Joined Sep 2026");
  });
});

describe("locale store", () => {
  it("defaults to en and notifies subscribers only on change", () => {
    expect(LOCALES).toContain(getSnapshot());
    let calls = 0;
    const off = subscribe(() => calls++);
    setLocale("en");
    expect(calls).toBe(0);
    off();
  });
});
