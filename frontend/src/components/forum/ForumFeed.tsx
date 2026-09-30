"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { t } from "@/lib/i18n";
import { useLocale } from "@/lib/i18n-client";
import {
  forumApi,
  forumErrorMessage,
  type ForumFeedQuery,
  type ForumPost,
  type ForumViewer,
} from "@/lib/forum";
import { ForumPostCard } from "@/components/forum/ForumPostCard";

interface ForumFeedProps {
  initialPosts: ForumPost[];
  initialHasMore: boolean;
  query: ForumFeedQuery;
  viewer: ForumViewer | null;
}

// Feed list with infinite scroll: the server renders the first page, and
// more pages load when the sentinel scrolls into view (with a button
// fallback). The parent remounts it (via `key`) whenever filters change.
export function ForumFeed({ initialPosts, initialHasMore, query, viewer }: ForumFeedProps) {
  useLocale();
  const [posts, setPosts] = useState(initialPosts);
  const [hasMore, setHasMore] = useState(initialHasMore);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const sentinel = useRef<HTMLDivElement>(null);
  // Offset into the server's ordering. A deleted post also leaves the
  // server's list, so deleting steps the offset back to avoid skipping one.
  const offset = useRef(initialPosts.length);

  const loadMore = useCallback(async () => {
    if (loading || !hasMore) return;
    setLoading(true);
    setError(null);
    try {
      const page = await forumApi.listPosts(query, offset.current);
      offset.current += page.posts.length;
      // "hot" ordering can shift between requests; skip anything already shown.
      setPosts((prev) => {
        const seen = new Set(prev.map((p) => p.id));
        return [...prev, ...page.posts.filter((p) => !seen.has(p.id))];
      });
      setHasMore(page.has_more);
    } catch (e) {
      setError(forumErrorMessage(e));
    } finally {
      setLoading(false);
    }
  }, [hasMore, loading, query]);

  useEffect(() => {
    const el = sentinel.current;
    if (!el || !hasMore) return;
    const io = new IntersectionObserver(
      (entries) => {
        if (entries.some((e) => e.isIntersecting)) void loadMore();
      },
      { rootMargin: "400px" },
    );
    io.observe(el);
    return () => io.disconnect();
  }, [hasMore, loadMore]);

  if (posts.length === 0) {
    return <p className="py-16 text-center text-sm text-muted">{t("forum_empty_feed")}</p>;
  }

  return (
    <div>
      {posts.map((post) => (
        <ForumPostCard
          key={post.id}
          post={post}
          viewer={viewer}
          showSection={!query.section}
          onDeleted={() => {
            offset.current = Math.max(0, offset.current - 1);
            setPosts((prev) => prev.filter((p) => p.id !== post.id));
          }}
        />
      ))}
      {hasMore && (
        <div ref={sentinel} className="flex justify-center py-6">
          <button
            type="button"
            onClick={() => void loadMore()}
            disabled={loading}
            className="rounded-md border border-divider px-4 py-2 text-sm text-muted hover:text-ink disabled:opacity-60"
          >
            {loading ? t("loading") : t("forum_load_more")}
          </button>
        </div>
      )}
      {error && <p className="pb-6 text-center text-sm text-danger">{error}</p>}
    </div>
  );
}
