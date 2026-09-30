import { apiJSON } from "@/lib/api-server";
import {
  feedSearchParams,
  type ForumFeedPage,
  type ForumFeedQuery,
  type ForumPost,
  type ForumTrendingItem,
} from "@/lib/forum";

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

export async function getForumPost(id: string): Promise<ForumPost | null> {
  return apiJSON<ForumPost>(`/api/forum/posts/${encodeURIComponent(id)}`);
}
