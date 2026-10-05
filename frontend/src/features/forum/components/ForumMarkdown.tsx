import ReactMarkdown, { type Components } from "react-markdown";
import remarkGfm from "remark-gfm";
import rehypeHighlight from "rehype-highlight";
import { cn } from "@/shared/lib/cn";
import { ForumCodeBlock } from "@/features/forum/components/ForumCodeBlock";
import { remarkMentions } from "@/features/forum/mentions";

interface ForumMarkdownProps {
  source: string;
  className?: string;
  // Comments are denser than posts: smaller headings and tighter spacing.
  compact?: boolean;
}

// Renders user-written forum markdown: GitHub-flavored (tables, task lists,
// strikethrough, autolinks) with highlighted fenced code and @mentions.
// react-markdown never renders raw HTML and drops unsafe URLs (javascript:
// etc.), so user input can't inject markup. Works in both server and client
// components.
export function ForumMarkdown({ source, className, compact }: ForumMarkdownProps) {
  return (
    <div className={cn("forum-md min-w-0 break-words", compact && "forum-md-compact", className)}>
      <ReactMarkdown
        remarkPlugins={[remarkGfm, remarkMentions]}
        rehypePlugins={[[rehypeHighlight, { detect: false }]]}
        components={components}
      >
        {source}
      </ReactMarkdown>
    </div>
  );
}

const components: Components = {
  a: ({ href, children }) => (
    <a href={href} target="_blank" rel="nofollow noopener noreferrer ugc">
      {children}
    </a>
  ),
  // eslint-disable-next-line @next/next/no-img-element -- arbitrary user-hosted images
  img: ({ src, alt }) => <img src={typeof src === "string" ? src : undefined} alt={alt ?? ""} loading="lazy" />,
  pre: ({ children }) => <ForumCodeBlock>{children}</ForumCodeBlock>,
  table: ({ children }) => (
    <div className="forum-md-table">
      <table>{children}</table>
    </div>
  ),
};
