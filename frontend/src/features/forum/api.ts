// Community forum types + browser-side API calls. Server components fetch
// through features/forum/server.ts instead (it forwards the session cookie).

import { t, type LocaleKey } from "@/shared/i18n";

import { apiRequest } from "@/shared/api/client";
import { ApiError } from "@/shared/api/errors";
import { formatShortDate } from "@/shared/format";

// Sections follow LeetCode Discuss; "interview" is Interview Experience.
export type ForumSection =
  | "interview-question"
  | "interview"
  | "compensation"
  | "career"
  | "study-guide"
  | "general"
  | "feedback";
export type ForumSort = "hot" | "votes" | "newest";
export type ForumCommentSort = "best" | "newest";

export const FORUM_SECTIONS: ForumSection[] = [
  "interview-question",
  "interview",
  "compensation",
  "career",
  "study-guide",
  "general",
  "feedback",
];

// Sections where posting (and commenting) anonymously is allowed.
export const FORUM_ANON_SECTIONS: ForumSection[] = ["interview-question", "interview", "compensation"];

export const FORUM_SECTION_LABEL_KEY: Record<ForumSection, LocaleKey> = {
  "interview-question": "forum_section_interview_question_label",
  interview: "forum_section_interview_label",
  compensation: "forum_section_compensation_label",
  career: "forum_section_career_label",
  "study-guide": "forum_section_study_guide_label",
  general: "forum_section_general_label",
  feedback: "forum_section_feedback_label",
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
  edited_at: string | null;
  // A deleted top-level comment kept as a placeholder because it still has
  // replies; its body is empty and author null.
  is_deleted: boolean;
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
    apiRequest<ForumFeedPage>(`/api/forum/posts?${feedSearchParams(query, offset, limit)}`),
  createPost: (input: ForumPostInput) =>
    apiRequest<ForumPost>("/api/forum/posts", { method: "POST", json: input }),
  updatePost: (id: string, input: ForumPostInput) =>
    apiRequest<ForumPost>(`/api/forum/posts/${encodeURIComponent(id)}`, {
      method: "PUT",
      json: input,
    }),
  deletePost: (id: string) =>
    apiRequest<void>(`/api/forum/posts/${encodeURIComponent(id)}`, { method: "DELETE" }),
  votePost: (id: string, value: -1 | 0 | 1) =>
    apiRequest<VoteResult>(`/api/forum/posts/${encodeURIComponent(id)}/vote`, {
      method: "POST",
      json: { value },
    }),
  pinPost: (id: string, pinned: boolean) =>
    apiRequest<{ is_pinned: boolean }>(`/api/forum/posts/${encodeURIComponent(id)}/pin`, {
      method: "POST",
      json: { pinned },
    }),
  listComments: (postId: string, sort: ForumCommentSort, offset: number, limit = 20) => {
    const params = new URLSearchParams({ sort, limit: String(limit) });
    if (offset > 0) params.set("offset", String(offset));
    return apiRequest<ForumCommentPage>(
      `/api/forum/posts/${encodeURIComponent(postId)}/comments?${params}`,
    );
  },
  createComment: (postId: string, body: string, parentId: string | null, isAnonymous: boolean) =>
    apiRequest<ForumComment>(`/api/forum/posts/${encodeURIComponent(postId)}/comments`, {
      method: "POST",
      json: { body, parent_id: parentId, is_anonymous: isAnonymous },
    }),
  updateComment: (id: string, body: string) =>
    apiRequest<ForumComment>(`/api/forum/comments/${encodeURIComponent(id)}`, {
      method: "PUT",
      json: { body },
    }),
  recordView: (postId: string) =>
    apiRequest<{ view_count: number }>(`/api/forum/posts/${encodeURIComponent(postId)}/view`, {
      method: "POST",
    }),
  voteComment: (id: string, value: -1 | 0 | 1) =>
    apiRequest<VoteResult>(`/api/forum/comments/${encodeURIComponent(id)}/vote`, {
      method: "POST",
      json: { value },
    }),
  deleteComment: (id: string) =>
    apiRequest<void>(`/api/forum/comments/${encodeURIComponent(id)}`, { method: "DELETE" }),
};

// Maps backend error codes to user-facing messages.
const ERROR_KEY: Record<string, LocaleKey> = {
  unauthorized: "forum_err_login_required",
  login_required: "forum_err_login_required",
  invalid_title: "forum_err_invalid_title",
  invalid_body: "forum_err_invalid_body",
  invalid_tag: "forum_err_invalid_tag",
  too_many_tags: "forum_err_too_many_tags",
  unknown_problem: "forum_err_unknown_problem",
  anonymous_not_allowed: "forum_err_anonymous_not_allowed",
  not_found: "forum_err_not_found",
  rate_limited: "forum_err_rate_limited",
  invalid_section: "forum_err_invalid_section",
};

export function forumErrorMessage(err: unknown): string {
  if (err instanceof ApiError && ERROR_KEY[err.code]) return t(ERROR_KEY[err.code]);
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
  return formatShortDate(iso);
}

