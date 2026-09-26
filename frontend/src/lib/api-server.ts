import { cookies } from "next/headers";

// Server-side fetch helper. Forwards the browser's cookies so the Go API can
// read the auth cookie. Always bypasses Next's data cache (per-request fresh).
const API_URL = process.env.CODRITIUM_API_URL ?? "http://localhost:8080";

export async function apiFetch(path: string, init: RequestInit = {}): Promise<Response> {
  const jar = await cookies();
  const cookieHeader = jar.getAll().map((c) => `${c.name}=${c.value}`).join("; ");
  return fetch(`${API_URL}${path}`, {
    ...init,
    headers: {
      ...(init.headers || {}),
      ...(cookieHeader ? { Cookie: cookieHeader } : {}),
    },
    cache: "no-store",
  });
}

// Convenience: parse JSON or return null on 401/403/404. 401 means
// "no cookie / not signed in", 403 means a gate refused (e.g. the
// reply endpoint when the submission has not been graded yet), 404
// means the resource doesn't exist. Pages decide whether to redirect
// to /login or show an empty state based on the null return.
export async function apiJSON<T>(path: string): Promise<T | null> {
  const r = await apiFetch(path);
  if (r.status === 401 || r.status === 403 || r.status === 404) return null;
  if (!r.ok) throw new Error(`${r.status} ${path}`);
  return (await r.json()) as T;
}
