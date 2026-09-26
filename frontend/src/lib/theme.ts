import { cookies } from "next/headers";

export const THEMES = ["default", "bright"] as const;
export type Theme = (typeof THEMES)[number];
export const DEFAULT_THEME: Theme = "default";

const COOKIE_NAME = "codritium_theme";

export async function getTheme(): Promise<Theme> {
  const jar = await cookies();
  const v = jar.get(COOKIE_NAME)?.value;
  return (THEMES as readonly string[]).includes(v ?? "") ? (v as Theme) : DEFAULT_THEME;
}

export async function setTheme(theme: Theme): Promise<void> {
  const jar = await cookies();
  jar.set(COOKIE_NAME, theme, {
    httpOnly: false,
    sameSite: "lax",
    path: "/",
    maxAge: 60 * 60 * 24 * 365,
  });
}

export const THEME_LABELS: Record<Theme, { label: string; tagline: string }> = {
  default: { label: "Default", tagline: "Geist neutral, system-aware light/dark." },
  bright: { label: "Bright", tagline: "White marble, violet accent, airy rhythm." },
};
