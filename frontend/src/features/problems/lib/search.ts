// Problem-list filters: the page URL (/problems?...) and the backend search
// query (/api/problems/search?...) are both derived from one ProblemFilters
// value, so the server-rendered first page and the client's scroll fetches
// can never disagree about what is being listed.
import { CATEGORIES, DIFFICULTIES, type Category, type Difficulty } from "@/shared/labels";

export type UserStatus = "todo" | "attempted" | "solved";
export const USER_STATUSES: UserStatus[] = ["todo", "attempted", "solved"];

export const PAGE_SIZE = 50;

export interface ProblemFilters {
  category?: Category;
  difficulty?: Difficulty;
  status?: UserStatus;
  tag?: string;
  q?: string;
}

export type FilterKey = keyof ProblemFilters;

function oneOf<T extends string>(allowed: readonly T[], value: string | undefined): T | undefined {
  return allowed.includes(value as T) ? (value as T) : undefined;
}

// Unknown enum values are dropped rather than passed through, so a stale or
// hand-edited URL shows the unfiltered list instead of an empty one.
export function parseFilters(sp: Partial<Record<FilterKey, string>>): ProblemFilters {
  return {
    category: oneOf(CATEGORIES, sp.category),
    difficulty: oneOf(DIFFICULTIES, sp.difficulty),
    status: oneOf(USER_STATUSES, sp.status),
    tag: sp.tag?.trim() || undefined,
    q: sp.q?.trim() || undefined,
  };
}

export function hasFilters(f: ProblemFilters): boolean {
  return Object.values(f).some(Boolean);
}

function pageParams(f: ProblemFilters): URLSearchParams {
  const params = new URLSearchParams();
  if (f.category) params.set("category", f.category);
  if (f.difficulty) params.set("difficulty", f.difficulty);
  if (f.status) params.set("status", f.status);
  if (f.tag) params.set("tag", f.tag);
  if (f.q) params.set("q", f.q);
  return params;
}

export function problemsHref(f: ProblemFilters): string {
  const qs = pageParams(f).toString();
  return qs ? `/problems?${qs}` : "/problems";
}

// The backend calls the per-user filter `user_status`; its own `status` is
// the admin-only publication state.
export function searchQuery(f: ProblemFilters, offset = 0, limit = PAGE_SIZE): string {
  const params = pageParams(f);
  if (f.status) {
    params.delete("status");
    params.set("user_status", f.status);
  }
  params.set("limit", String(limit));
  if (offset > 0) params.set("offset", String(offset));
  return params.toString();
}
