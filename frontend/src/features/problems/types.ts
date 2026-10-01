export type { Category, Difficulty } from "@/shared/labels";
import type { Category, Difficulty } from "@/shared/labels";

export interface Problem {
  id: string;
  title: string;
  category: Category;
  difficulty: Difficulty;
  description_md: string;
  canva_fuzzification_prefix_md: string | null;
  starter_files: Record<string, string>;
  hint: string | null;
  is_pro_only: boolean;
  ray_seed: boolean;
  company_slugs: string[];
}
