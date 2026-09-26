// The editor surface itself lives in components/ide/EditorPane.tsx now;
// this shim only re-exports the OpenTab type for paths that imported it
// from the old location. New code should import from @/lib/types directly.
export type { OpenTab } from "@/lib/types";
