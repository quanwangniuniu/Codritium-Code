// Browser-side endpoints. Transport (credentials, JSON, ApiError) lives in
// shared/api/client.ts.
import { API_BASE, apiFetch, apiRequest } from "@/shared/api/client";

export type SessionResp = {
  session_id: string;
  created: boolean;
  started_at: string;
  challenge_id: string;
};

export type DecisionKind = "approve" | "reject" | "modify";

export type TipsMessage = {
  seq: number;
  role: "user" | "model";
  text: string;
  created_at: string;
};

export type TipsStreamCallbacks = {
  onDelta: (delta: string) => void;
  onDone: () => void;
  onError: (msg: string) => void;
};

export const workspaceApi = {
  createSession: (challengeSlug: string, forceNew = false): Promise<SessionResp> =>
    apiRequest<SessionResp>("/api/sessions", {
      method: "POST",
      json: { challenge_slug: challengeSlug, force_new: forceNew },
    }),
  createSubmission: (input: {
    problemSlug: string;
    variant: string;
    codeFiles: Record<string, string>;
    sessionId: string | null;
  }): Promise<{ id: string }> =>
    apiRequest<{ id: string }>("/api/submissions", {
      method: "POST",
      json: {
        problem_slug: input.problemSlug,
        variant: input.variant,
        code_files: input.codeFiles,
        session_id: input.sessionId,
      },
    }),
  chat: (
    sessionId: string,
    message: string,
    files?: Record<string, string>,
  ): Promise<{ turn_index: number; accepted: boolean; session_id: string }> =>
    apiRequest("/api/chat/v2", {
      method: "POST",
      json: { session_id: sessionId, message, files: files ?? {} },
    }),
  decision: (
    sessionId: string,
    toolUseId: string,
    decision: DecisionKind,
    opts?: { modifiedInput?: string; reason?: string; comment?: string },
  ): Promise<void> =>
    apiRequest<void>("/api/decision", {
      method: "POST",
      json: {
        session_id: sessionId,
        tool_use_id: toolUseId,
        decision,
        modified_input: opts?.modifiedInput ?? "",
        reason: opts?.reason ?? "",
        comment: opts?.comment ?? "",
      },
    }),
  streamURL: (sessionId: string) =>
    `${API_BASE}/api/sessions/${encodeURIComponent(sessionId)}/stream`,
  tipsMessages: (sessionId: string) =>
    apiRequest<{ messages: TipsMessage[] }>(
      `/api/tips/messages?session_id=${encodeURIComponent(sessionId)}`,
    ),
  tipsStream: async (
    sessionId: string,
    message: string,
    fileContents: Record<string, string> | undefined,
    callbacks: TipsStreamCallbacks,
    signal?: AbortSignal,
  ): Promise<void> => {
    const res = await apiFetch("/api/tips", {
      method: "POST",
      json: {
        session_id: sessionId,
        message,
        file_contents: fileContents,
      },
      signal,
    });
    if (!res.ok || !res.body) {
      const text = await res.text().catch(() => "");
      callbacks.onError(text || `tips request failed: ${res.status}`);
      return;
    }
    const reader = res.body.getReader();
    const decoder = new TextDecoder();
    let buffer = "";
    while (true) {
      const { done, value } = await reader.read();
      if (done) break;
      buffer += decoder.decode(value, { stream: true });
      let idx: number;
      while ((idx = buffer.indexOf("\n\n")) !== -1) {
        const frame = buffer.slice(0, idx);
        buffer = buffer.slice(idx + 2);
        const line = frame.split("\n").find((l) => l.startsWith("data:"));
        if (!line) continue;
        const json = line.slice(5).trim();
        if (!json) continue;
        try {
          const evt = JSON.parse(json) as {
            delta?: string;
            done?: boolean;
            error?: string;
          };
          if (evt.error) {
            callbacks.onError(evt.error);
            return;
          }
          if (evt.delta) callbacks.onDelta(evt.delta);
          if (evt.done) {
            callbacks.onDone();
            return;
          }
        } catch (parseErr) {
          callbacks.onError(
            `bad SSE payload: ${(parseErr as Error).message}`,
          );
          return;
        }
      }
    }
    callbacks.onDone();
  },
};
