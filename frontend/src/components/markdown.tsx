import { cn } from "@/lib/utils";

interface MarkdownProps {
  source: string;
  className?: string;
}

function escape(s: string): string {
  return s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
}

function inlineFormat(s: string): string {
  return escape(s)
    .replace(
      /`([^`]+)`/g,
      '<code class="rounded bg-surface-2 px-1.5 py-0.5 text-[0.85em] font-mono">$1</code>',
    )
    .replace(/\*\*([^*]+)\*\*/g, "<strong>$1</strong>")
    .replace(/\*([^*]+)\*/g, "<em>$1</em>");
}

export function Markdown({ source, className }: MarkdownProps) {
  const lines = source.split("\n");
  const blocks: string[] = [];
  let i = 0;
  while (i < lines.length) {
    const line = lines[i];
    if (line.startsWith("## ")) {
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
    } else if (/^\d+\.\s/.test(line)) {
      const items: string[] = [];
      while (i < lines.length && /^\d+\.\s/.test(lines[i])) {
        items.push(`<li>${inlineFormat(lines[i].replace(/^\d+\.\s/, ""))}</li>`);
        i++;
      }
      blocks.push(`<ol class="my-3 ml-6 list-decimal space-y-1">${items.join("")}</ol>`);
    } else if (line.startsWith("- ")) {
      const items: string[] = [];
      while (i < lines.length && lines[i].startsWith("- ")) {
        items.push(`<li>${inlineFormat(lines[i].slice(2))}</li>`);
        i++;
      }
      blocks.push(`<ul class="my-3 ml-6 list-disc space-y-1">${items.join("")}</ul>`);
    } else if (line.trim() === "") {
      i++;
    } else {
      const para: string[] = [];
      while (
        i < lines.length &&
        lines[i].trim() !== "" &&
        !lines[i].startsWith("#") &&
        !lines[i].startsWith("- ") &&
        !/^\d+\.\s/.test(lines[i])
      ) {
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
