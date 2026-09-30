// Single source of truth for the enum -> i18n key / colour mappings that
// several pages render (problem category + difficulty, rubric dimensions,
// anti-patterns). Helpers fall back to the raw value for unknown keys so a
// new backend enum never renders as an empty string.

import { t, type LocaleKey } from "@/shared/i18n";
import type { Category, Difficulty } from "@/lib/types";

export const CATEGORIES: Category[] = [
  "debugging",
  "feature_build",
  "refactoring",
  "security",
  "company_premium",
];

export const DIFFICULTIES: Difficulty[] = ["easy", "medium", "hard"];

export const CATEGORY_LABEL_KEY: Record<Category, LocaleKey> = {
  debugging: "profile_category_debugging",
  feature_build: "profile_category_feature_build",
  refactoring: "profile_category_refactoring",
  security: "profile_category_security",
  company_premium: "profile_category_company_premium",
};

export const DIFFICULTY_LABEL_KEY: Record<Difficulty, LocaleKey> = {
  easy: "problems_difficulty_easy",
  medium: "problems_difficulty_medium",
  hard: "problems_difficulty_hard",
};

// Semantic tone, used by <Badge tone=...> and the app-palette text colours.
export const DIFFICULTY_TONE: Record<Difficulty, "success" | "warning" | "danger"> = {
  easy: "success",
  medium: "warning",
  hard: "danger",
};

// Text colour on app surfaces (problem list).
export const DIFFICULTY_TEXT_CLASS: Record<Difficulty, string> = {
  easy: "text-success",
  medium: "text-warning",
  hard: "text-danger",
};

// Text colour on the home / profile palette.
export const DIFFICULTY_HOME_TEXT_CLASS: Record<Difficulty, string> = {
  easy: "text-home-teal",
  medium: "text-home-amber",
  hard: "text-home-rose",
};

// Rubric dimensions. The backend stores the verification dimension as
// "verification"; the submission adapter renames it "verification_quality".
// Both spellings map to the same label.
export const DIMENSION_ORDER = [
  "correctness",
  "problem_decomposition",
  "ai_collaboration",
  "verification_quality",
  "communication",
] as const;

export const DIMENSION_LABEL_KEY: Record<string, LocaleKey> = {
  correctness: "dim_label_correctness",
  problem_decomposition: "dim_label_decomposition",
  ai_collaboration: "dim_label_ai_collab",
  verification: "dim_label_verification",
  verification_quality: "dim_label_verification",
  communication: "dim_label_communication",
};

export const ANTI_PATTERN_LABEL_KEY: Record<string, LocaleKey> = {
  hands_off: "antipattern_label_hands_off",
  feature_marathon: "antipattern_label_feature_marathon",
  ai_showcase: "antipattern_label_ai_showcase",
  not_thinking: "antipattern_label_not_thinking",
};

function labelFrom(map: Record<string, LocaleKey>, value: string): string {
  const key = map[value];
  return key ? t(key) : value;
}

export const categoryLabel = (c: string) => labelFrom(CATEGORY_LABEL_KEY, c);
export const difficultyLabel = (d: string) => labelFrom(DIFFICULTY_LABEL_KEY, d);
export const dimensionLabel = (d: string) => labelFrom(DIMENSION_LABEL_KEY, d);
export const antiPatternLabel = (n: string) => labelFrom(ANTI_PATTERN_LABEL_KEY, n);
