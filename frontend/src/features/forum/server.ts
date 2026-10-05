import { cache } from "react";
import { apiJSON } from "@/shared/api/server";
import {
  feedSearchParams,
  type ForumTagCount,
  type MyForumComment,
  type ForumFeedPage,
  type ForumFeedQuery,
  type ForumPost,
  type ForumTrendingItem,
} from "@/features/forum/api";

// Server-side forum reads for page components. The forum is public, so a
// null from apiJSON only ever means "not found" here.

export const FORUM_PAGE_SIZE = 20;

export async function getForumFeed(query: ForumFeedQuery): Promise<ForumFeedPage> {
  const page = await apiJSON<ForumFeedPage>(
    `/api/forum/posts?${feedSearchParams(query, 0, FORUM_PAGE_SIZE)}`,
  );
  return page ?? { posts: [], has_more: false };
}

export async function getPinnedForumPosts(): Promise<ForumPost[]> {
  return (await apiJSON<{ posts: ForumPost[] }>("/api/forum/pinned"))?.posts ?? [];
}

export async function getTrendingForumPosts(): Promise<ForumTrendingItem[]> {
  return (await apiJSON<{ posts: ForumTrendingItem[] }>("/api/forum/trending"))?.posts ?? [];
}

// Most-used tags, or just the one tag q when exact (for a tag page header).
export async function getForumTags(q = "", limit = 15): Promise<ForumTagCount[]> {
  const params = new URLSearchParams({ q, limit: String(limit) });
  return (await apiJSON<{ tags: ForumTagCount[] }>(`/api/forum/tags?${params}`))?.tags ?? [];
}

export async function getRelatedForumPosts(id: string): Promise<ForumPost[]> {
  return (await apiJSON<{ posts: ForumPost[] }>(`/api/forum/posts/${encodeURIComponent(id)}/related`))?.posts ?? [];
}

// The signed-in user's own comments, newest first.
export async function getMyForumComments(limit = 50): Promise<MyForumComment[]> {
  return (await apiJSON<{ comments: MyForumComment[] }>(`/api/forum/my-comments?limit=${limit}`))?.comments ?? [];
}

// The signed-in viewer's saved posts, most recently saved first.
export async function getForumBookmarks(limit = 50): Promise<ForumPost[]> {
  return (await apiJSON<ForumFeedPage>(`/api/forum/bookmarks?limit=${limit}`))?.posts ?? [];
}

// Cached per request so the post page and its metadata share one fetch.
export const getForumPost = cache(async (id: string): Promise<ForumPost | null> => {
  return apiJSON<ForumPost>(`/api/forum/posts/${encodeURIComponent(id)}`);
});
