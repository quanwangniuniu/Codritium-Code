// Browser-side endpoints. Transport (credentials, JSON, ApiError) lives in
// shared/api/client.ts.
import { apiRequest } from "@/shared/api/client";

export const profileApi = {
  updateMe: (input: { display_name: string; bio: string; region: string }): Promise<unknown> =>
    apiRequest("/api/me", { method: "PATCH", json: input }),
};
