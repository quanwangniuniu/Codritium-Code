import { describe, expect, it } from "vitest";
import type { ProblemSession } from "./session-store";
import {
  applyFileChange,
  markPatchResolved,
  panesOf,
  parseToolPath,
  patchFromProposal,
  updateTextMessage,
} from "./workspace-state";
import type { ChatMessage, StreamEnvelope } from "../types";

function session(over: Partial<ProblemSession> = {}): ProblemSession {
  return {
    fileContents: { "a.py": "a", "b.py": "b" },
    originalContents: { "a.py": "a", "b.py": "b" },
    openTabs: [{ name: "a.py", dirty: false }],
    activeTab: "a.py",
    messages: [],
    startedAt: 0,
    submissionId: null,
    ...over,
  };
}

describe("panesOf", () => {
  it("derives a single main pane from legacy sessions", () => {
    expect(panesOf(session())).toEqual([{ id: "main", tabs: [{ name: "a.py", dirty: false }], activeTab: "a.py" }]);
  });

  it("prefers persisted editorPanes", () => {
    const editorPanes = [{ id: "p1", tabs: [], activeTab: null }];
    expect(panesOf(session({ editorPanes }))).toBe(editorPanes);
  });
});

describe("applyFileChange", () => {
  const split = session({
    editorPanes: [
      { id: "main", tabs: [{ name: "a.py", dirty: false }], activeTab: "a.py" },
      { id: "p2", tabs: [{ name: "a.py", dirty: false }, { name: "b.py", dirty: false }], activeTab: "b.py" },
    ],
  });

  it("writes content and flags the tab dirty in every pane", () => {
    const next = applyFileChange(split, "a.py", "changed");
    expect(next.fileContents["a.py"]).toBe("changed");
    expect(next.editorPanes?.map((p) => p.tabs.find((t) => t.name === "a.py")?.dirty)).toEqual([true, true]);
    expect(next.openTabs).toEqual(next.editorPanes?.[0].tabs);
  });

  it("clears the dirty flag when content returns to the starter", () => {
    const dirty = applyFileChange(split, "a.py", "changed");
    expect(applyFileChange(dirty, "a.py", "a").editorPanes?.[0].tabs[0].dirty).toBe(false);
  });

  it("honours an explicit dirty flag", () => {
    expect(applyFileChange(split, "a.py", "a", true).editorPanes?.[0].tabs[0].dirty).toBe(true);
  });

  it("does not mutate the previous session", () => {
    applyFileChange(split, "a.py", "changed");
    expect(split.fileContents["a.py"]).toBe("a");
  });
});

describe("tool proposals", () => {
  it("parses Edit / Read summaries", () => {
    expect(parseToolPath("Edit src/app.py (+3 -1)")).toBe("src/app.py");
    expect(parseToolPath("Read README.md")).toBe("README.md");
    expect(parseToolPath("RunCommand pytest")).toBeNull();
  });

  it("builds a PendingPatch with the current file as diff baseline", () => {
    const env: StreamEnvelope = {
      session_id: "s1",
      seq: 3,
      kind: "tool_use_proposed",
      emitted_at: "",
      payload: { tool: "FileEdit", tool_use_id: "tu1", input_summary: "Edit a.py", auto: false },
    };
    expect(patchFromProposal(env, { "a.py": "x" })).toEqual({
      sessionId: "s1",
      toolUseId: "tu1",
      tool: "FileEdit",
      inputSummary: "Edit a.py",
      path: "a.py",
      oldContent: "x",
      newContent: "x",
      auto: false,
    });
  });
});

describe("message helpers", () => {
  const pending = { sessionId: "s", toolUseId: "tu1", tool: "FileEdit", inputSummary: "" };
  const messages: ChatMessage[] = [
    { id: "u1", role: "user", content: "hi" },
    { id: "a1", role: "assistant", content: "", streaming: true },
    { kind: "patch", id: "p-tu1", pending },
  ];

  it("marks the matching patch resolved", () => {
    const out = markPatchResolved(messages, "tu1", "reject");
    expect(out[2]).toEqual({ kind: "patch", id: "p-tu1", pending, resolved: { kind: "reject" } });
    expect(out[0]).toBe(messages[0]);
  });

  it("updates only the targeted text message", () => {
    const out = updateTextMessage(messages, "a1", (m) => ({ ...m, content: m.content + "yo" }));
    expect(out[1]).toMatchObject({ content: "yo", streaming: true });
    expect(out[2]).toBe(messages[2]);
  });
});
