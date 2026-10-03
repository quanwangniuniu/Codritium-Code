// The UI's view of the signed-in user (adapted from GET /api/me by
// features/auth/server.ts).
export interface User {
  id: string;
  handle: string;
  display_name: string;
  avatar_url: string;
  github_handle: string;
  region: string;
  streak_current: number;
  streak_longest: number;
  streak_last_completed_at: string | null;
  is_pro: boolean;
  tier?: "standard" | "pro" | "max";
  credits?: number;
  role?: "user" | "admin";
  email?: string;
}

// Raw GET /api/me payload as the browser sees it.
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
  // "" when unset.
  gender: "" | "male" | "female" | "non_binary" | "other";
  // YYYY-MM-DD, or "" when unset.
  birthday: string;
  website_url: string;
  github_url: string;
  linkedin_url: string;
  x_url: string;
};
