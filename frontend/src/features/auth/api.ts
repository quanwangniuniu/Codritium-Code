// Browser-side endpoints. Transport (credentials, JSON, ApiError) lives in
// shared/api/client.ts.
import { apiFetch, apiRequest } from "@/shared/api/client";
import type { Me } from "./types";

export const authApi = {
  me: () => apiRequest<Me>("/api/me"),
  logout: () => apiFetch("/api/auth/logout", { method: "POST" }),
};
