// Browser-side API client. Every request carries credentials so the
// codritium_user cookie travels between the frontend and the Go backend.
// Server components use shared/api/server.ts instead (it forwards the
// incoming request's cookies).

import { apiErrorFromResponse } from "./errors";

export const API_BASE = process.env.NEXT_PUBLIC_API_BASE || "http://localhost:8080";

export interface ApiRequestInit extends Omit<RequestInit, "body"> {
  // Serialised as JSON with a Content-Type header.
  json?: unknown;
  body?: BodyInit | null;
  // Treat 404 as success (resolves undefined). Used by idempotent deletes.
  allowNotFound?: boolean;
}

// Merges caller headers over the defaults. Unlike a naive
// `{ headers: {...}, ...init }` spread, caller-supplied `init.headers`
// never wipes the JSON Content-Type.
export function buildHeaders(init: ApiRequestInit): Headers {
  const headers = new Headers(init.headers);
  if (init.json !== undefined && !headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json");
  }
  return headers;
}

// Low-level: returns the raw Response for callers that stream the body.
// Does not throw on non-2xx.
export function apiFetch(path: string, init: ApiRequestInit = {}): Promise<Response> {
  const { json, allowNotFound: _allowNotFound, ...rest } = init;
  void _allowNotFound;
  return fetch(`${API_BASE}${path}`, {
    credentials: "include",
    ...rest,
    headers: buildHeaders(init),
    body: json !== undefined ? JSON.stringify(json) : rest.body,
  });
}

// JSON request: throws ApiError on non-2xx, resolves undefined on 204.
export async function apiRequest<T = unknown>(path: string, init: ApiRequestInit = {}): Promise<T> {
  const res = await apiFetch(path, init);
  if (!res.ok) {
    if (init.allowNotFound && res.status === 404) return undefined as T;
    throw await apiErrorFromResponse(res);
  }
  if (res.status === 204) return undefined as T;
  const text = await res.text();
  return (text ? JSON.parse(text) : undefined) as T;
}
