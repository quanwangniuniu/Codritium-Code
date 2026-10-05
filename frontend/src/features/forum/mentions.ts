// @mention parsing shared by the renderer and the autocomplete. The pattern
// matches the backend's (internal/community/forum/notify.go): a handle
// after "@" at the start or after a non-word character, so emails don't
// count.

export const MENTION_PATTERN = /(^|[^\w@])@([A-Za-z0-9][A-Za-z0-9_.-]{0,38})/g;

// Mention being typed just before the caret: "@" at the start or after a
// non-word character, then handle characters up to the caret.
const TYPING_PATTERN = /(?:^|[^\w@])@([A-Za-z0-9_.-]{0,39})$/;

export interface ActiveMention {
  query: string;
  // Index of the "@" in the text.
  start: number;
}

export function activeMention(text: string, caret: number): ActiveMention | null {
  const m = TYPING_PATTERN.exec(text.slice(0, caret));
  if (!m) return null;
  return { query: m[1], start: caret - m[1].length - 1 };
}

// Minimal mdast shapes; only what the plugin touches.
interface MdNode {
  type: string;
  value?: string;
  children?: MdNode[];
  data?: { hName?: string; hProperties?: Record<string, unknown> };
}

function splitMentions(value: string): MdNode[] | null {
  const out: MdNode[] = [];
  let last = 0;
  for (const m of value.matchAll(MENTION_PATTERN)) {
    const handle = m[2].replace(/[.-]+$/, "");
    if (!handle) continue;
    const at = m.index + m[1].length;
    if (at > last) out.push({ type: "text", value: value.slice(last, at) });
    out.push({
      type: "emphasis",
      data: { hName: "span", hProperties: { className: ["forum-mention"] } },
      children: [{ type: "text", value: `@${handle}` }],
    });
    last = at + 1 + handle.length;
  }
  if (out.length === 0) return null;
  if (last < value.length) out.push({ type: "text", value: value.slice(last) });
  return out;
}

function walk(node: MdNode) {
  if (!node.children || node.type === "link" || node.type === "linkReference") return;
  const next: MdNode[] = [];
  for (const child of node.children) {
    const parts = child.type === "text" && child.value ? splitMentions(child.value) : null;
    if (parts) next.push(...parts);
    else {
      walk(child);
      next.push(child);
    }
  }
  node.children = next;
}

// remark plugin: wraps @handles in <span class="forum-mention">. Code spans
// and blocks are separate node types, so mentions inside code stay as-is.
export function remarkMentions() {
  return (tree: MdNode) => walk(tree);
}
