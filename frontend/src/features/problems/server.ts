// Server-side problem reads (cookie-forwarding). Adapts the backend
// problem shapes into the UI's Problem type.
import { apiJSON } from "@/shared/api/server";
import type { Problem } from "./types";

interface BackendProblemBrief {
  slug: string;
  title: string;
  category: string;
  difficulty: string;
  requires_pro?: boolean;
}

interface BackendProblemFull extends BackendProblemBrief {
  readme_md: string;
  starter_files: Record<string, string>;
  variant: string;
  strip_variant: string;
}

function adaptProblemBrief(p: BackendProblemBrief): Problem {
  return {
    id: p.slug,
    title: p.title,
    category: p.category as Problem["category"],
    difficulty: p.difficulty as Problem["difficulty"],
    description_md: "",
    canva_fuzzification_prefix_md: null,
    starter_files: {},
    hint: null,
    is_pro_only: p.requires_pro ?? false,
    ray_seed: false,
    company_slugs: [],
  };
}

function adaptProblemFull(p: BackendProblemFull): Problem {
  return {
    id: p.slug,
    title: p.title,
    category: p.category as Problem["category"],
    difficulty: p.difficulty as Problem["difficulty"],
    description_md: p.readme_md,
    canva_fuzzification_prefix_md: null,
    starter_files: p.starter_files ?? {},
    hint: null,
    is_pro_only: p.requires_pro ?? false,
    ray_seed: false,
    company_slugs: [],
  };
}

export async function listProblems(): Promise<Problem[]> {
  const out = (await apiJSON<BackendProblemBrief[]>("/api/problems")) ?? [];
  return out.map(adaptProblemBrief);
}

export interface AttemptedProblem {
  slug: string;
  last_started_at: string;
}

interface BackendAttemptedProblem {
  challenge_slug: string;
  last_started_at: string;
}

export async function listMyAttemptedProblems(): Promise<AttemptedProblem[]> {
  const out = (await apiJSON<BackendAttemptedProblem[]>("/api/me/attempted")) ?? [];
  return out.map((p) => ({
    slug: p.challenge_slug,
    last_started_at: p.last_started_at,
  }));
}

export async function getProblem(id: string): Promise<Problem | null> {
  const p = await apiJSON<BackendProblemFull>(
    `/api/problems/${encodeURIComponent(id)}`,
  );
  return p ? adaptProblemFull(p) : null;
}
