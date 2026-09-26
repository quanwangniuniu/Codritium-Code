import { cookies } from "next/headers";

export const LAYOUT_PRESETS = [
  "ide-3pane",
  "doc-2pane",
  "terminal-bottom",
  "focus-read",
] as const;
export type LayoutPreset = (typeof LAYOUT_PRESETS)[number];
export const DEFAULT_LAYOUT_PRESET: LayoutPreset = "ide-3pane";

const COOKIE_NAME = "codritium_layout_preset";

export async function getLayoutPreset(): Promise<LayoutPreset> {
  const jar = await cookies();
  const v = jar.get(COOKIE_NAME)?.value;
  return (LAYOUT_PRESETS as readonly string[]).includes(v ?? "")
    ? (v as LayoutPreset)
    : DEFAULT_LAYOUT_PRESET;
}

export async function setLayoutPreset(preset: LayoutPreset): Promise<void> {
  const jar = await cookies();
  jar.set(COOKIE_NAME, preset, {
    httpOnly: false,
    sameSite: "lax",
    path: "/",
    maxAge: 60 * 60 * 24 * 365,
  });
}

export const LAYOUT_PRESET_LABELS: Record<
  LayoutPreset,
  { label: string; tagline: string }
> = {
  "ide-3pane": {
    label: "IDE",
    tagline:
      "Three-pane workspace: activity bar + sidebar (files / discussion / solutions) + editor + AI chat panel.",
  },
  "doc-2pane": {
    label: "Doc",
    tagline:
      "Two-pane reader-first layout: description tabs on the left, editor and tests on the right.",
  },
  "terminal-bottom": {
    label: "Terminal",
    tagline:
      "Problem and starter on top, agent prompt block transcript on the bottom.",
  },
  "focus-read": {
    label: "Focus",
    tagline:
      "Single centered column for deep reading; AI chat opens on demand from a corner button.",
  },
};
