// Browser-side endpoints. Transport (credentials, JSON, ApiError) lives in
// shared/api/client.ts.
import { apiRequest } from "@/shared/api/client";

export type ProblemListItem = {
  slug: string;
  title: string;
  category: string;
  difficulty: string;
  tags: string[];
  status: "draft" | "published" | "disabled";
  requires_pro: boolean;
};

// One row of GET /api/problems/search: catalog metadata plus the caller's
// progress on the problem.
export type ProblemSearchItem = ProblemListItem & {
  user_status: "todo" | "attempted" | "solved";
};

export type ProblemSearchPage = {
  items: ProblemSearchItem[];
  total: number;
  // null on the last page.
  next_offset: number | null;
};

export type FacetCount = { value: string; count: number };

export type ProblemFacets = {
  total: number;
  solved: number;
  tags: FacetCount[];
  categories: FacetCount[];
};

export type ProblemDetail = ProblemListItem & {
  readme_md: string;
  variant: "as-is" | "stripped";
  strip_variant: "as-is-only" | "both" | "stripped-only";
  starter_files: Record<string, string>;
  sample_test_filename?: string;
  sample_test_content?: string;
};

export type Comment = {
  id: string;
  problem_slug: string;
  user_id: string;
  user_handle: string;
  user_display_name: string;
  user_avatar_url: string;
  user_tier: "standard" | "pro" | "max";
  body: string;
  upvotes: number;
  downvotes: number;
  parent_id?: string;
  created_at: string;
  my_vote: -1 | 0 | 1;
};

export type CommentPage = {
  comments: Comment[];
  next_cursor: string;
};

export const problemsApi = {
  // `query` comes from lib/search.ts searchQuery().
  search: (query: string) => apiRequest<ProblemSearchPage>(`/api/problems/search?${query}`),
  getProblem: (slug: string, variant: "as-is" | "stripped" = "as-is") =>
    apiRequest<ProblemDetail>(`/api/problems/${slug}?variant=${variant}`),
  listComments: (slug: string, cursor?: string, limit = 20): Promise<CommentPage> => {
    const params = new URLSearchParams({ limit: String(limit) });
    if (cursor) params.set("cursor", cursor);
    return apiRequest<CommentPage>(`/api/problems/${encodeURIComponent(slug)}/comments?${params}`);
  },
  createComment: (slug: string, body: string, parentId?: string): Promise<{ id: string; created_at: string }> =>
    apiRequest(`/api/problems/${encodeURIComponent(slug)}/comments`, {
      method: "POST",
      json: { body, parent_id: parentId },
    }),
  voteComment: (id: string, value: -1 | 0 | 1): Promise<{ upvotes: number; downvotes: number; my_vote: number }> =>
    apiRequest(`/api/comments/${encodeURIComponent(id)}/vote`, {
      method: "POST",
      json: { value },
    }),
  deleteComment: (id: string): Promise<void> =>
    apiRequest<void>(`/api/comments/${encodeURIComponent(id)}`, {
      method: "DELETE",
      allowNotFound: true,
    }),
};
