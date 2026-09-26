// Backend API client. All requests include credentials so the
// codritium_user cookie travels between frontend (:3000) and backend (:8080).

const API_BASE = process.env.NEXT_PUBLIC_API_BASE || "http://localhost:8080";

export async function api<T = unknown>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    credentials: "include",
    headers: { "Content-Type": "application/json", ...(init?.headers || {}) },
    ...init,
  });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(`API ${path} ${res.status}: ${text}`);
  }
  return res.json() as Promise<T>;
}

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

export type ReplyEnvelope = {
  session_id?: string;
  seq?: number;
  kind: string;
  emitted_at?: string;
  payload: unknown;
};

export type ReplyResponse = {
  source: "official_ai" | "user_session";
  challenge_slug: string;
  generator_model?: string;
  candidate_index?: number;
  created_at?: string;
  session_id?: string;
  started_at?: string;
  envelopes: ReplyEnvelope[];
  files?: Record<string, string>;
  explanation_md?: string;
  starter_files?: Record<string, string>;
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
  me: () => api<Me>("/api/me"),
  switchUser: (handle: string) =>
    fetch(`${API_BASE}/api/auth/switch?handle=${encodeURIComponent(handle)}`, {
      method: "POST",
      credentials: "include",
    }),
  logout: () =>
    fetch(`${API_BASE}/api/auth/logout`, {
      method: "POST",
      credentials: "include",
    }),
  listProblems: () => api<ProblemListItem[]>("/api/problems"),
  getProblem: (slug: string, variant: "as-is" | "stripped" = "as-is") =>
    api<ProblemDetail>(`/api/problems/${slug}?variant=${variant}`),
  deleteSubmission: async (id: string): Promise<void> => {
    const res = await fetch(
      `${API_BASE}/api/submissions/${encodeURIComponent(id)}`,
      { method: "DELETE", credentials: "include" },
    );
    if (!res.ok && res.status !== 404) {
      const text = await res.text();
      throw new Error(`DELETE submission ${id} ${res.status}: ${text}`);
    }
  },
  createSession: (challengeSlug: string, forceNew = false): Promise<SessionResp> =>
    api<SessionResp>("/api/sessions", {
      method: "POST",
      body: JSON.stringify({ challenge_slug: challengeSlug, force_new: forceNew }),
    }),
  chat: (
    sessionId: string,
    message: string,
    files?: Record<string, string>,
  ): Promise<{ turn_index: number; accepted: boolean; session_id: string }> =>
    api("/api/chat/v2", {
      method: "POST",
      body: JSON.stringify({ session_id: sessionId, message, files: files ?? {} }),
    }),
  decision: async (toolUseId: string, decision: DecisionKind, opts?: { modifiedInput?: string; reason?: string; comment?: string }): Promise<void> => {
    const res = await fetch(`${API_BASE}/api/decision`, {
      method: "POST",
      credentials: "include",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        tool_use_id: toolUseId,
        decision,
        modified_input: opts?.modifiedInput ?? "",
        reason: opts?.reason ?? "",
        comment: opts?.comment ?? "",
      }),
    });
    if (!res.ok) {
      const text = await res.text();
      throw new Error(`POST decision ${res.status}: ${text}`);
    }
  },
  emitEvent: async (sessionId: string, kind: string, payload: Record<string, unknown>): Promise<void> => {
    const res = await fetch(`${API_BASE}/api/events`, {
      method: "POST",
      credentials: "include",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ session_id: sessionId, kind, payload }),
    });
    if (!res.ok) {
      const text = await res.text();
      throw new Error(`POST event ${res.status}: ${text}`);
    }
  },
  streamURL: (sessionId: string) =>
    `${API_BASE}/api/sessions/${encodeURIComponent(sessionId)}/stream`,
  officialReply: (slug: string): Promise<ReplyResponse> =>
    api<ReplyResponse>(`/api/challenges/${encodeURIComponent(slug)}/official-reply`),
  myReplay: (slug: string): Promise<ReplyResponse> =>
    api<ReplyResponse>(`/api/me/replays/${encodeURIComponent(slug)}`),
  listComments: (slug: string, cursor?: string, limit = 20): Promise<CommentPage> => {
    const params = new URLSearchParams({ limit: String(limit) });
    if (cursor) params.set("cursor", cursor);
    return api<CommentPage>(`/api/problems/${encodeURIComponent(slug)}/comments?${params}`);
  },
  createComment: (slug: string, body: string, parentId?: string): Promise<{ id: string; created_at: string }> =>
    api(`/api/problems/${encodeURIComponent(slug)}/comments`, {
      method: "POST",
      body: JSON.stringify({ body, parent_id: parentId }),
    }),
  voteComment: (id: string, value: -1 | 0 | 1): Promise<{ upvotes: number; downvotes: number; my_vote: number }> =>
    api(`/api/comments/${encodeURIComponent(id)}/vote`, {
      method: "POST",
      body: JSON.stringify({ value }),
    }),
  deleteComment: async (id: string): Promise<void> => {
    const res = await fetch(`${API_BASE}/api/comments/${encodeURIComponent(id)}`, {
      method: "DELETE",
      credentials: "include",
    });
    if (!res.ok && res.status !== 404) {
      const text = await res.text();
      throw new Error(`DELETE comment ${id} ${res.status}: ${text}`);
    }
  },
  listNotes: (sessionId: string): Promise<Note[]> =>
    api<Note[]>(`/api/sessions/${encodeURIComponent(sessionId)}/notes`),
  createNote: (sessionId: string, body: string): Promise<{ id: string; session_id: string; created_at: string }> =>
    api(`/api/sessions/${encodeURIComponent(sessionId)}/notes`, {
      method: "POST",
      body: JSON.stringify({ body }),
    }),
  updateNote: async (id: string, body: string): Promise<void> => {
    const res = await fetch(`${API_BASE}/api/notes/${encodeURIComponent(id)}`, {
      method: "PUT",
      credentials: "include",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ body }),
    });
    if (!res.ok) {
      const text = await res.text();
      throw new Error(`PUT note ${res.status}: ${text}`);
    }
  },
  deleteNote: async (id: string): Promise<void> => {
    const res = await fetch(`${API_BASE}/api/notes/${encodeURIComponent(id)}`, {
      method: "DELETE",
      credentials: "include",
    });
    if (!res.ok && res.status !== 404) {
      const text = await res.text();
      throw new Error(`DELETE note ${id} ${res.status}: ${text}`);
    }
  },
  shareNote: (id: string): Promise<{ comment_id: string; challenge_slug: string }> =>
    api(`/api/notes/${encodeURIComponent(id)}/share`, { method: "POST" }),
  tipsMessages: (sessionId: string) =>
    api<{ messages: TipsMessage[] }>(
      `/api/tips/messages?session_id=${encodeURIComponent(sessionId)}`,
    ),
  tipsStream: async (
    sessionId: string,
    message: string,
    fileContents: Record<string, string> | undefined,
    callbacks: TipsStreamCallbacks,
    signal?: AbortSignal,
  ): Promise<void> => {
    const res = await fetch(`${API_BASE}/api/tips`, {
      method: "POST",
      credentials: "include",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        session_id: sessionId,
        message,
        file_contents: fileContents,
      }),
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

// 5 mock users available for the demo cookie switcher.
export const MOCK_USERS = [
  { handle: "john", display_name: "John Smith", avatar_color: "#7dd3fc" },
  { handle: "alice", display_name: "Alice Wang", avatar_color: "#c4b5fd" },
  { handle: "bob", display_name: "Bob Martinez", avatar_color: "#fcd34d" },
  { handle: "carol", display_name: "Carol Lee", avatar_color: "#86efac" },
  { handle: "dan", display_name: "Dan Patel", avatar_color: "#fca5a5" },
] as const;
