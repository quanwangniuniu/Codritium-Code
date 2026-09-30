// ApiError is the single error type thrown by both the browser and the
// server API clients. The Go backend answers errors with
//   {"error": "<snake_code>", "message": "<human text>", ...extra}
// (older handlers send only `error`, and http.Error sends plain text), so
// every field is best-effort:
//   - code:    the `error` string, or "" when absent
//   - message: `message`, else the code, else the raw text, else "HTTP <status>"
//   - body:    the parsed JSON object, the raw text, or null
export class ApiError extends Error {
  readonly status: number;
  readonly code: string;
  readonly body: unknown;

  constructor(status: number, code: string, message: string, body: unknown = null) {
    super(message || code || `HTTP ${status}`);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
    this.body = body;
  }
}

export function isApiError(err: unknown): err is ApiError {
  return err instanceof ApiError;
}

// True when `err` is an ApiError carrying `code` (optionally with a given
// status). Also matches legacy plain-text bodies that contain the code, so
// handlers not yet migrated to the JSON error shape keep working.
export function hasErrorCode(err: unknown, code: string, status?: number): boolean {
  if (!isApiError(err)) return false;
  if (status !== undefined && err.status !== status) return false;
  if (err.code === code) return true;
  return typeof err.body === "string" && err.body.includes(code);
}

// Builds an ApiError from a status + raw response text. Exported separately
// from fromResponse so it can be unit-tested without a Response object.
export function apiErrorFromText(status: number, text: string): ApiError {
  const trimmed = text.trim();
  let body: unknown = trimmed === "" ? null : trimmed;
  let code = "";
  let message = "";
  if (trimmed.startsWith("{")) {
    try {
      const parsed = JSON.parse(trimmed) as unknown;
      if (parsed && typeof parsed === "object") {
        body = parsed;
        const rec = parsed as Record<string, unknown>;
        if (typeof rec.error === "string") code = rec.error;
        if (typeof rec.message === "string") message = rec.message;
      }
    } catch {
      // Not JSON after all; keep the raw text.
    }
  }
  if (!message) message = code || trimmed || `HTTP ${status}`;
  return new ApiError(status, code, message, body);
}

export async function apiErrorFromResponse(res: Response): Promise<ApiError> {
  const text = await res.text().catch(() => "");
  return apiErrorFromText(res.status, text);
}
