// Community forum types + browser-side API calls. Server components fetch
// through lib/forum-server.ts instead (it forwards the session cookie).

import { t, type LocaleKey } from "@/lib/i18n";

const API_BASE = process.env.NEXT_PUBLIC_API_BASE || "http://localhost:8080";

export type ForumSection = "interview" | "career" | "compensation" | "feedback" | "problems";
export type ForumSort = "hot" | "votes" | "newest";
export type ForumCommentSort = "best" | "newest";

export const FORUM_SECTIONS: ForumSection[] = [
  "interview",
  "career",
  "compensation",
  "feedback",
  "problems",
];

// Sections where posting (and commenting) anonymously is allowed.
export const FORUM_ANON_SECTIONS: ForumSection[] = ["interview", "compensation"];

export const FORUM_SECTION_LABEL_KEY: Record<ForumSection, LocaleKey> = {
  interview: "forum_section_interview_label",
  career: "forum_section_career_label",
  compensation: "forum_section_compensation_label",
  feedback: "forum_section_feedback_label",
  problems: "forum_section_problems_label",
};

export const FORUM_LIMITS = {
  titleMin: 5,
  titleMax: 150,
  bodyMax: 20000,
  commentMax: 5000,
  maxTags: 5,
};

export interface ForumAuthor {
  id: string;
  handle: string;
  display_name: string;
  avatar_url: string;
  avatar_color: string;
  verified: boolean;
}

export interface ForumPost {
  id: string;
  section: ForumSection;
  problem_slug: string | null;
  title: string;
  excerpt: string;
  body_md?: string;
  tags: string[];
  is_anonymous: boolean;
  is_pinned: boolean;
  // null when the post is anonymous and the viewer is neither its author
  // nor an admin.
  author: ForumAuthor | null;
  is_mine: boolean;
  upvotes: number;
  downvotes: number;
  score: number;
  view_count: number;
  comment_count: number;
  my_vote: -1 | 0 | 1;
  created_at: string;
  updated_at: string;
}

export interface ForumComment {
  id: string;
  post_id: string;
  parent_id: string | null;
  body: string;
  is_anonymous: boolean;
  author: ForumAuthor | null;
  is_mine: boolean;
  is_op: boolean;
  upvotes: number;
  downvotes: number;
  score: number;
  my_vote: -1 | 0 | 1;
  created_at: string;
  replies?: ForumComment[];
}

export interface ForumTrendingItem {
  id: string;
  title: string;
  section: ForumSection;
  tag: string;
  view_count: number;
}

export interface ForumFeedPage {
  posts: ForumPost[];
  has_more: boolean;
}

export interface ForumCommentPage {
  comments: ForumComment[];
  has_more: boolean;
}

export interface ForumPostInput {
  section: ForumSection;
  title: string;
  body_md: string;
  tags: string[];
  is_anonymous: boolean;
  problem_slug: string | null;
}

export interface ForumFeedQuery {
  section?: ForumSection;
  sort?: ForumSort;
  q?: string;
  tag?: string;
}

// Minimal viewer info the client components need.
export interface ForumViewer {
  id: string;
  isAdmin: boolean;
}

export interface VoteResult {
  upvotes: number;
  downvotes: number;
  score: number;
  my_vote: -1 | 0 | 1;
}

export class ForumApiError extends Error {
  constructor(
    readonly status: number,
    readonly code: string,
  ) {
    super(code || `HTTP ${status}`);
  }
}

async function forumFetch<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    credentials: "include",
    ...init,
    headers: { "Content-Type": "application/json", ...(init?.headers || {}) },
  });
  if (!res.ok) {
    let code = "";
    try {
      code = ((await res.json()) as { error?: string }).error ?? "";
    } catch {
      // Non-JSON error body (e.g. a 500 from http.Error).
    }
    throw new ForumApiError(res.status, code);
  }
  if (res.status === 204) return undefined as T;
  return (await res.json()) as T;
}

export function feedSearchParams(query: ForumFeedQuery, offset = 0, limit = 20): URLSearchParams {
  const params = new URLSearchParams();
  if (query.section) params.set("section", query.section);
  if (query.sort && query.sort !== "hot") params.set("sort", query.sort);
  if (query.q) params.set("q", query.q);
  if (query.tag) params.set("tag", query.tag);
  if (offset > 0) params.set("offset", String(offset));
  params.set("limit", String(limit));
  return params;
}

export const forumApi = {
  listPosts: (query: ForumFeedQuery, offset: number, limit = 20) =>
    forumFetch<ForumFeedPage>(`/api/forum/posts?${feedSearchParams(query, offset, limit)}`),
  createPost: (input: ForumPostInput) =>
    forumFetch<ForumPost>("/api/forum/posts", { method: "POST", body: JSON.stringify(input) }),
  updatePost: (id: string, input: ForumPostInput) =>
    forumFetch<ForumPost>(`/api/forum/posts/${encodeURIComponent(id)}`, {
      method: "PUT",
      body: JSON.stringify(input),
    }),
  deletePost: (id: string) =>
    forumFetch<void>(`/api/forum/posts/${encodeURIComponent(id)}`, { method: "DELETE" }),
  votePost: (id: string, value: -1 | 0 | 1) =>
    forumFetch<VoteResult>(`/api/forum/posts/${encodeURIComponent(id)}/vote`, {
      method: "POST",
      body: JSON.stringify({ value }),
    }),
  pinPost: (id: string, pinned: boolean) =>
    forumFetch<{ is_pinned: boolean }>(`/api/forum/posts/${encodeURIComponent(id)}/pin`, {
      method: "POST",
      body: JSON.stringify({ pinned }),
    }),
  listComments: (postId: string, sort: ForumCommentSort, offset: number, limit = 20) => {
    const params = new URLSearchParams({ sort, limit: String(limit) });
    if (offset > 0) params.set("offset", String(offset));
    return forumFetch<ForumCommentPage>(
      `/api/forum/posts/${encodeURIComponent(postId)}/comments?${params}`,
    );
  },
  createComment: (postId: string, body: string, parentId: string | null, isAnonymous: boolean) =>
    forumFetch<ForumComment>(`/api/forum/posts/${encodeURIComponent(postId)}/comments`, {
      method: "POST",
      body: JSON.stringify({ body, parent_id: parentId, is_anonymous: isAnonymous }),
    }),
  voteComment: (id: string, value: -1 | 0 | 1) =>
    forumFetch<VoteResult>(`/api/forum/comments/${encodeURIComponent(id)}/vote`, {
      method: "POST",
      body: JSON.stringify({ value }),
    }),
  deleteComment: (id: string) =>
    forumFetch<void>(`/api/forum/comments/${encodeURIComponent(id)}`, { method: "DELETE" }),
};

// Maps backend error codes to user-facing messages.
const ERROR_KEY: Record<string, LocaleKey> = {
  login_required: "forum_err_login_required",
  invalid_title: "forum_err_invalid_title",
  invalid_body: "forum_err_invalid_body",
  invalid_tag: "forum_err_invalid_tag",
  too_many_tags: "forum_err_too_many_tags",
  unknown_problem: "forum_err_unknown_problem",
  anonymous_not_allowed: "forum_err_anonymous_not_allowed",
  not_found: "forum_err_not_found",
};

export function forumErrorMessage(err: unknown): string {
  if (err instanceof ForumApiError && ERROR_KEY[err.code]) return t(ERROR_KEY[err.code]);
  return t("forum_err_generic");
}

// LeetCode-style relative time: "just now", "5m ago", "3h ago", "4d ago",
// then an absolute date after a month.
export function forumTimeAgo(iso: string, now = Date.now()): string {
  const then = new Date(iso).getTime();
  const minutes = Math.floor((now - then) / 60000);
  if (minutes < 1) return t("forum_just_now");
  if (minutes < 60) return t("forum_minutes_ago_fmt", { params: { n: minutes } });
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return t("forum_hours_ago_fmt", { params: { n: hours } });
  const days = Math.floor(hours / 24);
  if (days < 30) return t("forum_days_ago_fmt", { params: { n: days } });
  return new Date(iso).toLocaleDateString("en-US", {
    month: "short",
    day: "numeric",
    year: "numeric",
  });
}

// 1234 -> "1.2K", 25100 -> "25.1K", like LeetCode's view counts.
export function compactCount(n: number): string {
  if (n < 1000) return String(n);
  if (n < 1_000_000) return `${(n / 1000).toFixed(1).replace(/\.0$/, "")}K`;
  return `${(n / 1_000_000).toFixed(1).replace(/\.0$/, "")}M`;
}
