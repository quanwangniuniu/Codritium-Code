// Codritium toast store — module-level listener pattern, framework-agnostic
// at the data layer; the React `ToastContainer` component subscribes.
//
// Usage:
//   import { toast } from "@/lib/toast";
//   import { t } from "@/lib/i18n";
//   toast.error(t("gemini_unavailable"));
//
// Rules:
//   - Never call with a bare literal string. Always pass t("key") so the
//     copy stays in i18n.ts and we keep English-first parity.
//   - Queue cap = 3; if more arrive, the oldest is dropped.

export type ToastVariant = "info" | "success" | "warn" | "error";

export interface ToastItem {
  id: number;
  variant: ToastVariant;
  message: string;
  // Auto-dismiss after this many ms; 0 disables auto-dismiss.
  durationMs: number;
}

type Listener = (items: ToastItem[]) => void;

const MAX_QUEUE = 3;
const DEFAULT_DURATION_MS = 5000;

let items: ToastItem[] = [];
let nextId = 1;
const listeners: Set<Listener> = new Set();

function emit(): void {
  // Pass a fresh array reference so React detects state change.
  const snapshot = [...items];
  for (const l of listeners) l(snapshot);
}

function push(variant: ToastVariant, message: string, durationMs: number = DEFAULT_DURATION_MS): number {
  const id = nextId++;
  items.push({ id, variant, message, durationMs });
  // Cap queue: drop the oldest.
  while (items.length > MAX_QUEUE) items.shift();
  emit();

  if (durationMs > 0) {
    setTimeout(() => dismiss(id), durationMs);
  }
  return id;
}

export function dismiss(id: number): void {
  const before = items.length;
  items = items.filter((it) => it.id !== id);
  if (items.length !== before) emit();
}

export function subscribe(listener: Listener): () => void {
  listeners.add(listener);
  // Push current snapshot immediately so subscribers don't miss state.
  listener([...items]);
  return () => {
    listeners.delete(listener);
  };
}

export const toast = {
  info: (message: string, durationMs?: number): number => push("info", message, durationMs),
  success: (message: string, durationMs?: number): number => push("success", message, durationMs),
  warn: (message: string, durationMs?: number): number => push("warn", message, durationMs),
  error: (message: string, durationMs?: number): number => push("error", message, durationMs),
};
