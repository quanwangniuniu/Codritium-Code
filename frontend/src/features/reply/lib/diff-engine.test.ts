import { describe, expect, it } from "vitest";
import {
  applyPatches,
  extractPatches,
  patchCursorFromEnvelopeSeq,
  type PatchExtractInput,
} from "./diff-engine";

const edit = (seq: number, file: string, before: string, after: string): PatchExtractInput => ({
  session_id: "s",
  seq,
  kind: "tool_use_proposed",
  payload: { tool_name: "FileEdit", file_path: file, patch: { before, after } },
});

describe("extractPatches", () => {
  it("keeps only well-formed FileEdit proposals, sorted by seq", () => {
    const envs: PatchExtractInput[] = [
      edit(5, "b.py", "x", "y"),
      { session_id: "s", seq: 2, kind: "tool_use_proposed", payload: { tool_name: "FileRead", file_path: "a.py" } },
      { session_id: "s", seq: 3, kind: "turn_completed", payload: {} },
      { session_id: "s", seq: 4, kind: "tool_use_proposed", payload: { tool_name: "FileEdit", file_path: "a.py", patch: { before: 1 } } },
      edit(1, "a.py", "1", "2"),
    ];
    expect(extractPatches(envs).map((p) => p.seq)).toEqual([1, 5]);
  });
});

describe("applyPatches", () => {
  const starter = { "a.py": "one\ntwo\n", "b.py": "x" };
  const patches = extractPatches([edit(1, "a.py", "two", "TWO"), edit(2, "b.py", "missing", "z"), edit(3, "c.py", "", "new")]);

  it("applies the first `count` patches cumulatively", () => {
    expect(applyPatches(starter, patches, 1)).toEqual({
      files: { "a.py": "one\nTWO\n", "b.py": "x" },
      lastTouchedFile: "a.py",
      failedSeqs: [],
    });
  });

  it("records unapplicable patches but still highlights their file", () => {
    const s = applyPatches(starter, patches, 2);
    expect(s.failedSeqs).toEqual([2]);
    expect(s.lastTouchedFile).toBe("b.py");
    expect(s.files["b.py"]).toBe("x");
  });

  it("creates files from an empty before and does not mutate the starter", () => {
    const s = applyPatches(starter, patches, 99);
    expect(s.files["c.py"]).toBe("new");
    expect(starter).toEqual({ "a.py": "one\ntwo\n", "b.py": "x" });
  });
});

describe("patchCursorFromEnvelopeSeq", () => {
  it("counts patches at or before the envelope seq", () => {
    const patches = extractPatches([edit(2, "a", "", "x"), edit(5, "a", "", "y")]);
    expect(patchCursorFromEnvelopeSeq(patches, 1)).toBe(0);
    expect(patchCursorFromEnvelopeSeq(patches, 2)).toBe(1);
    expect(patchCursorFromEnvelopeSeq(patches, 10)).toBe(2);
  });
});
