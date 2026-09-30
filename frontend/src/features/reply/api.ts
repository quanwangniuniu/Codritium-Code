// Browser-side endpoints. Transport (credentials, JSON, ApiError) lives in
// shared/api/client.ts.
import { apiRequest } from "@/shared/api/client";

export type Note = {
  id: string;
  session_id: string;
  body: string;
  shared_to_comment_id?: string;
  created_at: string;
  updated_at: string;
};

export const replyApi = {
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
};
