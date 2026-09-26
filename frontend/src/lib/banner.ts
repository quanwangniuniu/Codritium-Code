// Codritium banner store — top-of-page persistent status indicator.
//
// Unlike toast (transient notification), banner is for continuous state
// (e.g. "grader unavailable"). One banner at a time; show/hide controlled
// by feature code (typically the health check loop).

export type BannerVariant = "warn" | "error";

export interface BannerState {
  visible: boolean;
  variant: BannerVariant;
  message: string;
}

type Listener = (state: BannerState) => void;

const HIDDEN: BannerState = { visible: false, variant: "warn", message: "" };
let state: BannerState = HIDDEN;
const listeners: Set<Listener> = new Set();

function emit(): void {
  for (const l of listeners) l(state);
}

export function showBanner(variant: BannerVariant, message: string): void {
  state = { visible: true, variant, message };
  emit();
}

export function hideBanner(): void {
  if (!state.visible) return;
  state = HIDDEN;
  emit();
}

export function subscribeBanner(listener: Listener): () => void {
  listeners.add(listener);
  listener(state);
  return () => {
    listeners.delete(listener);
  };
}
