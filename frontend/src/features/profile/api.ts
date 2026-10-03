// Browser-side endpoints. Transport (credentials, JSON, ApiError) lives in
// shared/api/client.ts.
import { apiRequest } from "@/shared/api/client";
import type { Me } from "@/features/auth/types";

// The fields PATCH /api/me accepts. Omitted fields are left unchanged; ""
// clears an optional one.
export type ProfileUpdate = Partial<
  Pick<
    Me,
    "display_name" | "bio" | "region" | "gender" | "birthday" | "website_url" | "github_url" | "linkedin_url" | "x_url"
  >
>;

// Every call resolves to the updated /api/me payload.
export const profileApi = {
  updateMe: (input: ProfileUpdate) => apiRequest<Me>("/api/me", { method: "PATCH", json: input }),
  uploadAvatar: (image: Blob) =>
    apiRequest<Me>("/api/me/avatar", { method: "PUT", body: image, headers: { "Content-Type": image.type } }),
  deleteAvatar: () => apiRequest<Me>("/api/me/avatar", { method: "DELETE" }),
};
