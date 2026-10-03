import { describe, expect, it } from "vitest";

import { avatarSrc, defaultAvatarDataUri, defaultAvatarSvg } from "./defaultAvatar";

describe("defaultAvatarSvg", () => {
  it("is deterministic per seed", () => {
    expect(defaultAvatarSvg("u_0ebcfb4594")).toBe(defaultAvatarSvg("u_0ebcfb4594"));
  });

  it("differs between similar seeds", () => {
    const seen = new Set(Array.from({ length: 50 }, (_, i) => defaultAvatarSvg(`u_${i}`)));
    expect(seen.size).toBeGreaterThan(45);
  });

  it("always draws at least one cell, mirrored left to right", () => {
    for (const seed of ["", "a", "john", "u_2bb48268cd", "字"]) {
      const xs = [...defaultAvatarSvg(seed).matchAll(/<rect x="(\d+)" y="(\d+)"/g)].map((m) => `${m[1]},${m[2]}`);
      expect(xs.length).toBeGreaterThan(0);
      for (const cell of xs) {
        const [x, y] = cell.split(",").map(Number);
        expect(xs).toContain(`${80 - 12 - x},${y}`);
      }
    }
  });
});

describe("avatarSrc", () => {
  it("falls back to the generated avatar", () => {
    expect(avatarSrc("", "john")).toBe(defaultAvatarDataUri("john"));
    expect(avatarSrc(null, "john")).toMatch(/^data:image\/svg\+xml,/);
  });

  it("resolves API paths against the backend origin", () => {
    expect(avatarSrc("/api/users/1/avatar?v=2", "john")).toBe("http://localhost:8080/api/users/1/avatar?v=2");
  });

  it("leaves absolute URLs alone", () => {
    expect(avatarSrc("https://avatars.githubusercontent.com/u/1", "john")).toBe(
      "https://avatars.githubusercontent.com/u/1",
    );
  });
});
