import type { Category, Difficulty } from "@/features/problems/types";

// GET /api/me/profile — the LeetCode-style profile page payload.
export interface ProfileBucket {
  key: string;
  solved: number;
  total: number;
}

export interface ProfileCount {
  total: number;
  last_week: number;
}

export interface ProfileData {
  user: {
    handle: string;
    display_name: string;
    bio: string;
    region: string;
    tier: string;
    avatar_url: string;
    avatar_color: string;
    member_since: string;
  };
  solved: {
    solved: number;
    total: number;
    attempting: number;
    by_difficulty: ProfileBucket[];
    by_category: ProfileBucket[];
  };
  calendar: {
    days: { date: string; count: number }[];
    total_submissions: number;
    active_days: number;
    current_streak: number;
    max_streak: number;
  };
  dimensions: { dimension: string; average: number | null; samples: number }[];
  best_score: number | null;
  community: {
    posts: ProfileCount;
    comments: ProfileCount;
    upvotes: ProfileCount;
  };
  recent_submissions: {
    id: string;
    problem_slug: string;
    problem_title: string;
    difficulty: Difficulty;
    status: string;
    final_score: number | null;
    submitted_at: string;
  }[];
  solved_problems: {
    slug: string;
    title: string;
    category: Category;
    difficulty: Difficulty;
    best_score: number;
    solved_at: string;
  }[];
  recent_posts: {
    id: string;
    title: string;
    section: string;
    upvotes: number;
    comment_count: number;
    created_at: string;
  }[];
}
