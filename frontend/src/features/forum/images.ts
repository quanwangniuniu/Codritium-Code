// Image uploads for posts and comments: shrink in the browser, then store
// on the backend, which returns the URL to embed in markdown.

import { apiRequest } from "@/shared/api/client";

// Longest edge after resizing; plenty for a post column, and keeps files
// well under the backend's 1 MB cap.
const MAX_EDGE = 1600;
export const MAX_IMAGE_BYTES = 1 << 20;

export function isImageFile(f: File): boolean {
  return /^image\/(png|jpeg|webp|gif)$/.test(f.type);
}

// Scales an image down to MAX_EDGE and re-encodes it (WebP where the browser
// can, else JPEG). GIFs under the cap are sent as-is so animation survives;
// small images that are already a fine size are left alone.
async function shrink(file: File): Promise<Blob> {
  if (file.type === "image/gif" && file.size <= MAX_IMAGE_BYTES) return file;
  const bitmap = await createImageBitmap(file);
  const scale = Math.min(1, MAX_EDGE / Math.max(bitmap.width, bitmap.height));
  if (scale === 1 && file.size <= MAX_IMAGE_BYTES / 2) {
    bitmap.close();
    return file;
  }
  const canvas = document.createElement("canvas");
  canvas.width = Math.round(bitmap.width * scale);
  canvas.height = Math.round(bitmap.height * scale);
  const ctx = canvas.getContext("2d");
  if (!ctx) throw new Error("no 2d context");
  ctx.drawImage(bitmap, 0, 0, canvas.width, canvas.height);
  bitmap.close();
  const encode = (type: string) => new Promise<Blob | null>((resolve) => canvas.toBlob(resolve, type, 0.85));
  // Browsers that can't encode WebP hand back a PNG (or null) instead.
  const webp = await encode("image/webp");
  if (webp?.type === "image/webp") return webp;
  const jpeg = await encode("image/jpeg");
  if (!jpeg) throw new Error("could not encode image");
  return jpeg;
}

export async function uploadForumImage(file: File): Promise<string> {
  const blob = await shrink(file);
  const res = await apiRequest<{ id: string; url: string }>("/api/forum/images", {
    method: "POST",
    body: blob,
    headers: { "Content-Type": blob.type || "application/octet-stream" },
  });
  return res.url;
}

// Alt text from a file name: "my-diagram_v2.png" -> "my diagram v2".
export function altFromFileName(name: string): string {
  return name.replace(/\.[a-z0-9]+$/i, "").replace(/[-_]+/g, " ").replace(/[[\]]/g, "").trim() || "image";
}
