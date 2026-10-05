import { describe, expect, it } from "vitest";
import { activeMention, remarkMentions } from "@/features/forum/mentions";

describe("activeMention", () => {
  it("finds the handle being typed before the caret", () => {
    expect(activeMention("hi @al", 6)).toEqual({ query: "al", start: 3 });
    expect(activeMention("@", 1)).toEqual({ query: "", start: 0 });
    expect(activeMention("(@bob", 5)).toEqual({ query: "bob", start: 1 });
  });

  it("ignores emails, finished mentions, and text after the caret", () => {
    expect(activeMention("me@host", 7)).toBeNull();
    expect(activeMention("@bob done", 9)).toBeNull();
    expect(activeMention("@bob", 0)).toBeNull();
  });
});

type Node = { type: string; value?: string; children?: Node[]; data?: { hName?: string } };

function run(tree: Node): Node {
  remarkMentions()(tree as never);
  return tree;
}

describe("remarkMentions", () => {
  it("wraps @handles in mention spans and trims trailing punctuation", () => {
    const tree = run({ type: "root", children: [{ type: "paragraph", children: [{ type: "text", value: "thanks @ann_b. and a@b.com" }] }] });
    const parts = tree.children![0].children!;
    expect(parts.map((p) => p.type)).toEqual(["text", "emphasis", "text"]);
    expect(parts[1].data?.hName).toBe("span");
    expect(parts[1].children![0].value).toBe("@ann_b");
    expect(parts[2].value).toBe(". and a@b.com");
  });

  it("leaves code and links alone", () => {
    const code: Node = { type: "inlineCode", value: "@nope" };
    const link: Node = { type: "link", children: [{ type: "text", value: "@inlink" }] };
    const tree = run({ type: "root", children: [{ type: "paragraph", children: [code, link] }] });
    expect(tree.children![0].children).toEqual([code, link]);
  });
});
