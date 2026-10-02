import { describe, expect, it } from "vitest";
import type { ProblemSession } from "./session-store";
import {
  applyFileChange,
  applyToolResult,
  attachToolResult,
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

  it("builds a targeted FileEdit preview from old_text and new_text", () => {
    const env: StreamEnvelope = {
      session_id: "s1",
      seq: 3,
      kind: "tool_use_proposed",
      emitted_at: "",
      payload: {
        tool: "FileEdit",
        tool_use_id: "tu1",
        input_summary: "Edit a.py",
        input: {
          path: "a.py",
          old_text: "value = None",
          new_text: "value = \"fixed\"",
        },
        auto: false,
      },
    };

    expect(
      patchFromProposal(env, {
        "a.py": "before\nvalue = None\nafter\n",
      }),
    ).toEqual({
      sessionId: "s1",
      toolUseId: "tu1",
      tool: "FileEdit",
      inputSummary: "Edit a.py",
      path: "a.py",
      oldContent: "before\nvalue = None\nafter\n",
      newContent: "before\nvalue = \"fixed\"\nafter\n",
      auto: false,
    });
  });

  it("builds a new-file preview when old_text is empty", () => {
    const env: StreamEnvelope = {
      session_id: "s1",
      seq: 4,
      kind: "tool_use_proposed",
      emitted_at: "",
      payload: {
        tool: "FileEdit",
        tool_use_id: "tu2",
        input_summary: "Edit test_fix.py",
        input: {
          path: "test_fix.py",
          old_text: "",
          new_text: "def test_fix():\n    assert True\n",
        },
        auto: false,
      },
    };

    expect(patchFromProposal(env, {})).toMatchObject({
      path: "test_fix.py",
      oldContent: "",
      newContent: "def test_fix():\n    assert True\n",
    });
  });

  it("supports legacy full-file FileEdit proposals", () => {
    const env: StreamEnvelope = {
      session_id: "s1",
      seq: 5,
      kind: "tool_use_proposed",
      emitted_at: "",
      payload: {
        tool: "FileEdit",
        tool_use_id: "tu3",
        input_summary: "Edit a.py",
        input: {
          path: "a.py",
          content: "complete replacement",
        },
        auto: false,
      },
    };

    expect(
      patchFromProposal(env, { "a.py": "old content" }),
    ).toMatchObject({
      path: "a.py",
      oldContent: "old content",
      newContent: "complete replacement",
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

  it("attaches a tool result to the matching patch", () => {
    const out = attachToolResult(messages, "tu1", {
      summary: "1 passed",
      isError: false,
      durationMs: 125,
    });

    expect(out[2]).toMatchObject({
      kind: "patch",
      id: "p-tu1",
      toolResult: {
        summary: "1 passed",
        isError: false,
        durationMs: 125,
      },
    });
    expect(out[0]).toBe(messages[0]);
  });

  it("applies an approved FileEdit after a successful tool result", () => {
    const stagedMessages = markPatchResolved(
      messages,
      "tu1",
      "approve",
      { path: "new.py", content: "print('ok')" },
    );

    const currentSession = session({
      messages: stagedMessages,
    });

    const out = applyToolResult(currentSession, "tu1", {
      summary: "applied",
      isError: false,
    });

    expect(out.fileContents["new.py"]).toBe("print('ok')");
  });

  it("does not apply an approved FileEdit after a failed tool result", () => {
    const stagedMessages = markPatchResolved(
      messages,
      "tu1",
      "approve",
      { path: "test_answer.py", content: "hidden" },
    );

    const currentSession = session({
      messages: stagedMessages,
    });

    const out = applyToolResult(currentSession, "tu1", {
      summary: "hidden test file is protected",
      isError: true,
    });

    expect(out.fileContents["test_answer.py"]).toBeUndefined();
    expect(out.messages[2]).toMatchObject({
      toolResult: {
        isError: true,
      },
    });
  });

  it("updates only the targeted text message", () => {
    const out = updateTextMessage(messages, "a1", (m) => ({ ...m, content: m.content + "yo" }));
    expect(out[1]).toMatchObject({ content: "yo", streaming: true });
    expect(out[2]).toBe(messages[2]);
  });
});
