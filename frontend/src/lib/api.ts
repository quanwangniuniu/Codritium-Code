// Browser-side endpoint wrappers for the workspace / problems / profile
// surfaces. Transport (credentials, JSON, ApiError) lives in shared/api.

import { API_BASE, apiFetch, apiRequest } from "@/shared/api/client";

export type Me = {
  id: string;
  handle: string;
  email: string;
  display_name: string;
  region: string;
  tier: "standard" | "pro" | "max";
  credits: number;
  role: "user" | "admin";
  avatar_url: string;
  avatar_color: string;
  bio: string;
  is_pro: boolean;
};

export type ProblemListItem = {
  slug: string;
  title: string;
  category: string;
  difficulty: string;
  tags: string[];
  status: "draft" | "published" | "disabled";
  requires_pro: boolean;
};

export type ProblemDetail = ProblemListItem & {
  readme_md: string;
  variant: "as-is" | "stripped";
  strip_variant: "as-is-only" | "both" | "stripped-only";
  starter_files: Record<string, string>;
  sample_test_filename?: string;
  sample_test_content?: string;
};

export type Comment = {
  id: string;
  problem_slug: string;
  user_id: string;
  user_handle: string;
  user_display_name: string;
  user_avatar_url: string;
  user_tier: "standard" | "pro" | "max";
  body: string;
  upvotes: number;
  downvotes: number;
  parent_id?: string;
  created_at: string;
  my_vote: -1 | 0 | 1;
};

export type CommentPage = {
  comments: Comment[];
  next_cursor: string;
};

export type Note = {
  id: string;
  session_id: string;
  body: string;
  shared_to_comment_id?: string;
  created_at: string;
  updated_at: string;
};

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

export const Backend = {
  me: () => apiRequest<Me>("/api/me"),
  logout: () => apiFetch("/api/auth/logout", { method: "POST" }),
  getProblem: (slug: string, variant: "as-is" | "stripped" = "as-is") =>
    apiRequest<ProblemDetail>(`/api/problems/${slug}?variant=${variant}`),
  deleteSubmission: (id: string): Promise<void> =>
    apiRequest<void>(`/api/submissions/${encodeURIComponent(id)}`, {
      method: "DELETE",
      allowNotFound: true,
    }),
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
  updateMe: (input: { display_name: string; bio: string; region: string }): Promise<unknown> =>
    apiRequest("/api/me", { method: "PATCH", json: input }),
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
  listComments: (slug: string, cursor?: string, limit = 20): Promise<CommentPage> => {
    const params = new URLSearchParams({ limit: String(limit) });
    if (cursor) params.set("cursor", cursor);
    return apiRequest<CommentPage>(`/api/problems/${encodeURIComponent(slug)}/comments?${params}`);
  },
  createComment: (slug: string, body: string, parentId?: string): Promise<{ id: string; created_at: string }> =>
    apiRequest(`/api/problems/${encodeURIComponent(slug)}/comments`, {
      method: "POST",
      json: { body, parent_id: parentId },
    }),
  voteComment: (id: string, value: -1 | 0 | 1): Promise<{ upvotes: number; downvotes: number; my_vote: number }> =>
    apiRequest(`/api/comments/${encodeURIComponent(id)}/vote`, {
      method: "POST",
      json: { value },
    }),
  deleteComment: (id: string): Promise<void> =>
    apiRequest<void>(`/api/comments/${encodeURIComponent(id)}`, {
      method: "DELETE",
      allowNotFound: true,
    }),
  listNotes: (sessionId: string): Promise<Note[]> =>
    apiRequest<Note[]>(`/api/sessions/${encodeURIComponent(sessionId)}/notes`),
  createNote: (sessionId: string, body: string): Promise<{ id: string; session_id: string; created_at: string }> =>
    apiRequest(`/api/sessions/${encodeURIComponent(sessionId)}/notes`, {
      method: "POST",
      json: { body },
    }),
  updateNote: (id: string, body: string): Promise<void> =>
    apiRequest<void>(`/api/notes/${encodeURIComponent(id)}`, {
      method: "PUT",
      json: { body },
    }),
  deleteNote: (id: string): Promise<void> =>
    apiRequest<void>(`/api/notes/${encodeURIComponent(id)}`, {
      method: "DELETE",
      allowNotFound: true,
    }),
  shareNote: (id: string): Promise<{ comment_id: string; challenge_slug: string }> =>
    apiRequest(`/api/notes/${encodeURIComponent(id)}/share`, { method: "POST" }),
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
  apiBase: API_BASE,
};
