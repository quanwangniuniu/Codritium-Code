import { describe, expect, it } from "vitest";
import { ApiError, apiErrorFromResponse, apiErrorFromText, hasErrorCode } from "@/shared/api/errors";

describe("apiErrorFromText", () => {
  it("reads code + message from the JSON error shape", () => {
    const e = apiErrorFromText(
      401,
      JSON.stringify({ error: "unauthorized", message: "Sign in to continue.", extra: 1 }),
    );
    expect(e).toBeInstanceOf(ApiError);
    expect(e.status).toBe(401);
    expect(e.code).toBe("unauthorized");
    expect(e.message).toBe("Sign in to continue.");
    expect(e.body).toEqual({ error: "unauthorized", message: "Sign in to continue.", extra: 1 });
  });

  it("falls back to the code when there is no message", () => {
    const e = apiErrorFromText(400, '{"error":"invalid_title"}');
    expect(e.code).toBe("invalid_title");
    expect(e.message).toBe("invalid_title");
  });

  it("keeps plain-text bodies", () => {
    const e = apiErrorFromText(500, "internal error\n");
    expect(e.code).toBe("");
    expect(e.message).toBe("internal error");
    expect(e.body).toBe("internal error");
  });

  it("handles empty and malformed bodies", () => {
    expect(apiErrorFromText(502, "").message).toBe("HTTP 502");
    expect(apiErrorFromText(502, "").body).toBeNull();
    const bad = apiErrorFromText(400, "{not json");
    expect(bad.code).toBe("");
    expect(bad.body).toBe("{not json");
  });

  it("ignores a non-string error field", () => {
    expect(apiErrorFromText(400, '{"error":{"nested":true}}').code).toBe("");
  });
});

describe("apiErrorFromResponse", () => {
  it("parses a fetch Response", async () => {
    const res = new Response('{"error":"not_found","message":"Nope"}', { status: 404 });
    const e = await apiErrorFromResponse(res);
    expect([e.status, e.code, e.message]).toEqual([404, "not_found", "Nope"]);
  });
});

describe("hasErrorCode", () => {
  it("matches JSON codes and legacy plain-text bodies", () => {
    const json = apiErrorFromText(403, '{"error":"must_complete_problem","message":"Solve it first"}');
    const text = apiErrorFromText(403, "must_complete_problem");
    expect(hasErrorCode(json, "must_complete_problem", 403)).toBe(true);
    expect(hasErrorCode(text, "must_complete_problem", 403)).toBe(true);
    expect(hasErrorCode(json, "must_complete_problem", 409)).toBe(false);
    expect(hasErrorCode(new Error("must_complete_problem"), "must_complete_problem")).toBe(false);
  });
});
