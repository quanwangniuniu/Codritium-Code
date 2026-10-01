import { cookies } from "next/headers";

import { apiErrorFromResponse } from "./errors";

// Server-side fetch helper. Forwards the browser's cookies so the Go API can
// read the auth cookie. Always bypasses Next's data cache (per-request fresh).
const API_URL = process.env.CODRITIUM_API_URL ?? "http://localhost:8080";

export async function serverFetch(path: string, init: RequestInit = {}): Promise<Response> {
  const jar = await cookies();
  const cookieHeader = jar.getAll().map((c) => `${c.name}=${c.value}`).join("; ");
  const headers = new Headers(init.headers);
  if (cookieHeader) headers.set("Cookie", cookieHeader);
  return fetch(`${API_URL}${path}`, {
    ...init,
    headers,
    cache: "no-store",
  });
}

// Convenience: parse JSON or return null on 401/403/404. 401 means
// "no cookie / not signed in", 403 means a gate refused (e.g. the
// reply endpoint when the submission has not been graded yet), 404
// means the resource doesn't exist. Pages decide whether to redirect
// to /login or show an empty state based on the null return. Any other
// non-2xx throws an ApiError.
export async function apiJSON<T>(path: string): Promise<T | null> {
  const r = await serverFetch(path);
  if (r.status === 401 || r.status === 403 || r.status === 404) return null;
  if (!r.ok) throw await apiErrorFromResponse(r);
  return (await r.json()) as T;
}
