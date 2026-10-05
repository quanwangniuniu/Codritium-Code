"use client";

import { useEffect, useState } from "react";
import { Eye } from "lucide-react";
import { compactCount } from "@/shared/format";
import { forumApi } from "@/features/forum/api";

// Shows a post's view count and reports this read once the page is on
// screen. Counting from the browser (not the server render) means prefetches
// and crawlers that don't run scripts aren't counted; the backend dedupes
// repeat reads per reader per day.
export function ForumViewCount({ postId, initial }: { postId: string; initial: number }) {
  const [count, setCount] = useState(initial);

  useEffect(() => {
    let cancelled = false;
    forumApi
      .recordView(postId)
      .then((r) => {
        if (!cancelled) setCount(r.view_count);
      })
      .catch(() => {
        // A missed view isn't worth bothering the reader about.
      });
    return () => {
      cancelled = true;
    };
  }, [postId]);

  return (
    <span className="inline-flex items-center gap-1">
      <Eye size={14} />
      {compactCount(count)}
    </span>
  );
}
