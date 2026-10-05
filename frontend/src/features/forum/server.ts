import { cache } from "react";
import { apiJSON } from "@/shared/api/server";
import {
  feedSearchParams,
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

// The signed-in viewer's saved posts, most recently saved first.
export async function getForumBookmarks(limit = 50): Promise<ForumPost[]> {
  return (await apiJSON<ForumFeedPage>(`/api/forum/bookmarks?limit=${limit}`))?.posts ?? [];
}

// Cached per request so the post page and its metadata share one fetch.
export const getForumPost = cache(async (id: string): Promise<ForumPost | null> => {
  return apiJSON<ForumPost>(`/api/forum/posts/${encodeURIComponent(id)}`);
});
