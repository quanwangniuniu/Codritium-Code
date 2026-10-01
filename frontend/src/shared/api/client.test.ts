import { afterEach, describe, expect, it, vi } from "vitest";
import { API_BASE, apiRequest, buildHeaders } from "@/shared/api/client";
import { ApiError } from "@/shared/api/errors";

afterEach(() => {
  vi.unstubAllGlobals();
});

function stubFetch(res: Response) {
  const fn = vi.fn(async (url: string, init: RequestInit) => {
    void url;
    void init;
    return res;
  });
  vi.stubGlobal("fetch", fn);
  return fn;
}

describe("buildHeaders", () => {
  it("keeps the JSON content type when the caller adds headers", () => {
    const h = buildHeaders({ json: {}, headers: { "X-Trace": "1" } });
    expect(h.get("Content-Type")).toBe("application/json");
    expect(h.get("X-Trace")).toBe("1");
  });

  it("lets the caller override the content type", () => {
    expect(buildHeaders({ json: {}, headers: { "Content-Type": "text/plain" } }).get("Content-Type")).toBe(
      "text/plain",
    );
  });

  it("sends no content type without a JSON body", () => {
    expect(buildHeaders({ method: "DELETE" }).has("Content-Type")).toBe(false);
  });
});

describe("apiRequest", () => {
  it("sends credentials + a JSON body and parses the response", async () => {
    const fetchFn = stubFetch(new Response('{"ok":true}', { status: 200 }));
    const out = await apiRequest<{ ok: boolean }>("/api/x", { method: "POST", json: { a: 1 } });
    expect(out).toEqual({ ok: true });
    const [url, init] = fetchFn.mock.calls[0];
    expect(url).toBe(`${API_BASE}/api/x`);
    expect(init.credentials).toBe("include");
    expect(init.body).toBe('{"a":1}');
    expect(new Headers(init.headers).get("Content-Type")).toBe("application/json");
  });

  it("resolves undefined on 204", async () => {
    stubFetch(new Response(null, { status: 204 }));
    await expect(apiRequest("/api/x", { method: "DELETE" })).resolves.toBeUndefined();
  });

  it("throws ApiError with code and message", async () => {
    stubFetch(new Response('{"error":"invalid_vote","message":"Bad vote"}', { status: 400 }));
    const err = await apiRequest("/api/x").catch((e: unknown) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect(err).toMatchObject({ status: 400, code: "invalid_vote", message: "Bad vote" });
  });

  it("treats 404 as success when allowNotFound is set", async () => {
    stubFetch(new Response("gone", { status: 404 }));
    await expect(apiRequest("/api/x", { allowNotFound: true })).resolves.toBeUndefined();
  });
});
