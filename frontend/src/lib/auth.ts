import { apiJSON } from "@/lib/api-server";
import type { User } from "@/lib/types";

// Codritium backend returns a minimal user shape: {id, handle, display_name}.
// Map it onto the richer User shape this UI was originally written against,
// filling streak/region/pro/avatar fields with defaults until those features
// land server-side.
interface BackendUser {
  id: string;
  handle: string;
  display_name: string;
  region?: string;
  is_pro?: boolean;
  avatar_color?: string;
  avatar_url?: string;
  tier?: "standard" | "pro" | "max";
  credits?: number;
  role?: "user" | "admin";
  email?: string;
}

function adaptUser(u: BackendUser): User {
  return {
    id: u.id,
    display_name: u.display_name,
    avatar_url: u.avatar_url ?? "",
    github_handle: u.handle,
    region: u.region ?? "",
    streak_current: 0,
    streak_longest: 0,
    streak_last_completed_at: null,
    is_pro: u.is_pro ?? false,
    tier: u.tier ?? "standard",
    credits: u.credits ?? 0,
    role: u.role ?? "user",
    email: u.email,
  };
}

export async function currentUser(): Promise<User | null> {
  const u = await apiJSON<BackendUser>("/api/me");
  return u ? adaptUser(u) : null;
}

export { adaptUser };
export type { BackendUser };
