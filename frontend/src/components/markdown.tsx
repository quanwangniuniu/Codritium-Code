import { cn } from "@/lib/utils";

interface MarkdownProps {
  source: string;
  className?: string;
}

// Small, deliberately limited markdown renderer. It renders user-written
// forum posts too, so every piece of source text is HTML-escaped before any
// tag is added, and the only attribute built from user input is a link href
// restricted to http(s) URLs.

function escape(s: string): string {
  return s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
}

const CODE_TOKEN = "\u0000";

function inlineFormat(s: string): string {
  // Pull code spans out first so their contents are never treated as
  // emphasis or links, then restore them at the end.
  const codes: string[] = [];
  const withTokens = escape(s).replace(/`([^`]+)`/g, (_, code: string) => {
    codes.push(
      `<code class="rounded bg-surface-2 px-1.5 py-0.5 text-[0.85em] font-mono">${code}</code>`,
    );
    return `${CODE_TOKEN}${codes.length - 1}${CODE_TOKEN}`;
  });
  return withTokens
    .replace(
      /\[([^\]]+)\]\((https?:\/\/[^\s)]+)\)/g,
      (_, text: string, url: string) =>
        `<a href="${url.replace(/"/g, "&quot;").replace(/'/g, "&#39;")}" target="_blank" rel="nofollow noopener noreferrer" class="text-accent underline underline-offset-2">${text}</a>`,
    )
    .replace(/\*\*([^*]+)\*\*/g, "<strong>$1</strong>")
    .replace(/\*([^*]+)\*/g, "<em>$1</em>")
    .replace(new RegExp(`${CODE_TOKEN}(\\d+)${CODE_TOKEN}`, "g"), (_, n: string) => codes[Number(n)]);
}

const BULLET = /^[-*]\s/;
const ORDERED = /^\d+\.\s/;
const FENCE = /^```/;
const QUOTE = /^>\s?/;

function startsBlock(line: string): boolean {
  return (
    line.startsWith("#") ||
    BULLET.test(line) ||
    ORDERED.test(line) ||
    FENCE.test(line) ||
    QUOTE.test(line)
  );
}

export function Markdown({ source, className }: MarkdownProps) {
  const lines = source.split(/\r?\n/);
  const blocks: string[] = [];
  let i = 0;
  while (i < lines.length) {
    const line = lines[i];
    if (FENCE.test(line)) {
      const code: string[] = [];
      i++;
      while (i < lines.length && !FENCE.test(lines[i])) {
        code.push(lines[i]);
        i++;
      }
      i++; // closing fence (or end of input)
      blocks.push(
        `<pre class="my-3 overflow-x-auto rounded-md bg-surface-2 p-3 text-[0.85em] leading-relaxed text-ink"><code class="font-mono">${escape(code.join("\n"))}</code></pre>`,
      );
    } else if (line.startsWith("## ")) {
      blocks.push(
        `<h2 class="mt-6 mb-3 text-lg font-semibold tracking-tight text-ink">${inlineFormat(line.slice(3))}</h2>`,
      );
      i++;
    } else if (line.startsWith("### ")) {
      blocks.push(
        `<h3 class="mt-4 mb-2 text-base font-semibold text-ink">${inlineFormat(line.slice(4))}</h3>`,
      );
      i++;
    } else if (line.startsWith("# ")) {
      blocks.push(
        `<h1 class="mt-6 mb-4 text-2xl font-semibold tracking-tight text-ink">${inlineFormat(line.slice(2))}</h1>`,
      );
      i++;
    } else if (QUOTE.test(line)) {
      const quote: string[] = [];
      while (i < lines.length && QUOTE.test(lines[i])) {
        quote.push(lines[i].replace(QUOTE, ""));
        i++;
      }
      blocks.push(
        `<blockquote class="my-3 border-l-2 border-divider-strong pl-3 text-muted">${inlineFormat(quote.join(" "))}</blockquote>`,
      );
    } else if (ORDERED.test(line)) {
      const items: string[] = [];
      while (i < lines.length && ORDERED.test(lines[i])) {
        items.push(`<li>${inlineFormat(lines[i].replace(ORDERED, ""))}</li>`);
        i++;
      }
      blocks.push(`<ol class="my-3 ml-6 list-decimal space-y-1">${items.join("")}</ol>`);
    } else if (BULLET.test(line)) {
      const items: string[] = [];
      while (i < lines.length && BULLET.test(lines[i])) {
        items.push(`<li>${inlineFormat(lines[i].slice(2))}</li>`);
        i++;
      }
      blocks.push(`<ul class="my-3 ml-6 list-disc space-y-1">${items.join("")}</ul>`);
    } else if (line.trim() === "") {
      i++;
    } else {
      // Always consume the first line: a line like "#tag" matches no block
      // rule above but still starts with "#", and must not stall the loop.
      const para: string[] = [line];
      i++;
      while (i < lines.length && lines[i].trim() !== "" && !startsBlock(lines[i])) {
        para.push(lines[i]);
        i++;
      }
      blocks.push(
        `<p class="my-3 leading-relaxed">${inlineFormat(para.join(" "))}</p>`,
      );
    }
  }
  return (
    <div
      className={cn("text-sm text-muted", className)}
      dangerouslySetInnerHTML={{ __html: blocks.join("") }}
    />
  );
}
