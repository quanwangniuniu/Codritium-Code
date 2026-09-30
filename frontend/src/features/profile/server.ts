import { apiJSON } from "@/shared/api/server";
import type { ProfileData } from "./types";

export async function getMyProfile(): Promise<ProfileData | null> {
  return apiJSON<ProfileData>("/api/me/profile");
}
