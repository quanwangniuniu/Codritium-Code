import { cn } from "@/lib/utils";

interface CodeBlockProps {
  filename?: string;
  code: string;
  className?: string;
}

export function CodeBlock({ filename, code, className }: CodeBlockProps) {
  return (
    <div className={cn("rounded-lg border border-divider overflow-hidden", className)}>
      {filename && (
        <div className="border-b border-divider bg-surface-2 px-4 py-2 text-xs font-mono text-muted">
          {filename}
        </div>
      )}
      <pre className="bg-surface px-4 py-3 text-xs font-mono leading-relaxed overflow-x-auto text-ink">
        <code>{code}</code>
      </pre>
    </div>
  );
}
