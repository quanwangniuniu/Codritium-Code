// Generated default avatar: a symmetric 5x5 identicon derived from a seed
// (the user's handle). The same seed always yields the same picture, so a
// user without an uploaded avatar still looks the same everywhere, and no
// two users are told apart only by the first letter of their name.
import { API_BASE } from "@/shared/api/client";

const GRID = 5;
const CELL = 12;
const PAD = 10;
const SIZE = PAD * 2 + GRID * CELL;

// FNV-1a, 32-bit. Small, stable across browsers and Node, and spreads
// similar handles ("u_1", "u_2") well.
function hash(seed: string): number {
  let h = 0x811c9dc5;
  for (let i = 0; i < seed.length; i++) {
    h ^= seed.charCodeAt(i);
    h = Math.imul(h, 0x01000193);
  }
  return h >>> 0;
}

export function defaultAvatarSvg(seed: string): string {
  const h = hash(seed || "?");
  const hue = h % 360;
  // 15 bits decide the left three columns; the right two mirror them.
  const bits = hash(`${seed}:cells`);
  let cells = "";
  let filled = 0;
  for (let row = 0; row < GRID; row++) {
    for (let col = 0; col < 3; col++) {
      if (((bits >>> (row * 3 + col)) & 1) === 0) continue;
      filled++;
      for (const c of col === 2 ? [col] : [col, GRID - 1 - col]) {
        cells += `<rect x="${PAD + c * CELL}" y="${PAD + row * CELL}" width="${CELL}" height="${CELL}"/>`;
      }
    }
  }
  // An all-empty grid would be a blank square; show the centre cell instead.
  if (filled === 0) {
    cells = `<rect x="${PAD + 2 * CELL}" y="${PAD + 2 * CELL}" width="${CELL}" height="${CELL}"/>`;
  }
  return (
    `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 ${SIZE} ${SIZE}">` +
    `<rect width="${SIZE}" height="${SIZE}" fill="hsl(${hue} 60% 92%)"/>` +
    `<g fill="hsl(${hue} 55% 48%)">${cells}</g></svg>`
  );
}

export function defaultAvatarDataUri(seed: string): string {
  return `data:image/svg+xml,${encodeURIComponent(defaultAvatarSvg(seed))}`;
}

// The <img src> for a user. Uploaded avatars are stored as an API path
// ("/api/users/<id>/avatar?v=..."), which the browser must load from the
// backend origin; OAuth avatars are absolute URLs; no avatar at all falls
// back to the generated one.
export function avatarSrc(url: string | null | undefined, seed: string): string {
  if (!url) return defaultAvatarDataUri(seed);
  return url.startsWith("/") ? `${API_BASE}${url}` : url;
}
