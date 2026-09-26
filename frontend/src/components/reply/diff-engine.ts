// diff-engine maintains the cumulative file state across replay steps.
// Given the starter snapshot and the ordered list of patches up to the
// current cursor index, it returns the rendered content of each file at
// that moment, plus which file (if any) the latest applied patch touched.
//
// Patches that cannot be located inside the current content degrade per
// the 2026-05-22 product decision: we drop them silently from the
// cumulative state and the UI still highlights the file path from the
// originating envelope. The fallback keeps the timeline playable instead
// of breaking when the model emits a "before" that no longer matches.

export interface Patch {
  seq: number;
  file_path: string;
  before: string;
  after: string;
}

export interface PatchExtractInput {
  session_id: string;
  seq: number;
  kind: string;
  payload: Record<string, unknown>;
}

/**
 * Pull patches in chronological order from a stream of envelopes. Only
 * tool_use_proposed envelopes whose tool_name is FileEdit and whose
 * payload carries a {before, after} patch survive; everything else is
 * silently skipped.
 */
export function extractPatches(envelopes: PatchExtractInput[]): Patch[] {
  const out: Patch[] = [];
  for (const env of envelopes) {
    if (env.kind !== "tool_use_proposed") continue;
    const p = env.payload;
    if (p.tool_name !== "FileEdit") continue;
    const filePath = typeof p.file_path === "string" ? p.file_path : null;
    const patch = p.patch as { before?: unknown; after?: unknown } | undefined;
    if (!filePath || !patch) continue;
    if (typeof patch.before !== "string" || typeof patch.after !== "string") continue;
    out.push({
      seq: env.seq,
      file_path: filePath,
      before: patch.before,
      after: patch.after,
    });
  }
  return out.sort((a, b) => a.seq - b.seq);
}

export interface CumulativeState {
  // filename -> current content after applying all patches through cursor
  files: Record<string, string>;
  // most recently applied patch's file (drives the file-tree highlight)
  lastTouchedFile: string | null;
  // patches that failed to apply (before string not found) - kept for diagnostics
  failedSeqs: number[];
}

/**
 * Apply the first `count` patches (in seq order) on top of the starter
 * snapshot. `count` is the cursor index from the StepController - meaning
 * "show me the world after this many FileEdit envelopes have played".
 */
export function applyPatches(
  starter: Record<string, string>,
  patches: Patch[],
  count: number,
): CumulativeState {
  const files: Record<string, string> = { ...starter };
  const failedSeqs: number[] = [];
  let lastTouchedFile: string | null = null;

  for (let i = 0; i < Math.min(count, patches.length); i++) {
    const p = patches[i];
    const current = files[p.file_path] ?? "";
    const idx = current.indexOf(p.before);
    if (idx < 0) {
      failedSeqs.push(p.seq);
      // We still record the touched file - the UI uses this for the
      // file-tree highlight even when the patch cannot be applied.
      lastTouchedFile = p.file_path;
      continue;
    }
    files[p.file_path] = current.slice(0, idx) + p.after + current.slice(idx + p.before.length);
    lastTouchedFile = p.file_path;
  }

  return { files, lastTouchedFile, failedSeqs };
}

/**
 * Count how many FileEdit patches in `patches` precede or include the
 * given envelope seq. The replay UI knows the envelope cursor; the diff
 * viewer wants the patch cursor (a subset). This converts between them.
 */
export function patchCursorFromEnvelopeSeq(
  patches: Patch[],
  envelopeSeq: number,
): number {
  let n = 0;
  for (const p of patches) {
    if (p.seq <= envelopeSeq) n++;
    else break;
  }
  return n;
}
