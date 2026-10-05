"use client";

import { isValidElement, useRef, useState, type ReactNode } from "react";
import { Check, Copy } from "lucide-react";
import { t } from "@/shared/i18n";
import { useLocale } from "@/shared/i18n/client";

// A fenced code block with a language label and a copy button.
export function ForumCodeBlock({ children }: { children: ReactNode }) {
  useLocale();
  const ref = useRef<HTMLPreElement>(null);
  const [copied, setCopied] = useState(false);
  const codeClass = isValidElement<{ className?: string }>(children) ? children.props.className : undefined;
  const lang = codeClass?.match(/language-([\w+#-]+)/)?.[1];

  async function copy() {
    const text = ref.current?.innerText ?? "";
    try {
      await navigator.clipboard.writeText(text.replace(/\n$/, ""));
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch {
      // Clipboard can be blocked (insecure origin, permissions); nothing to do.
    }
  }

  return (
    <div className="forum-md-code group relative">
      <pre ref={ref}>{children}</pre>
      <div className="absolute right-2 top-2 flex items-center gap-2 text-xs text-faint">
        {lang && <span className="opacity-100 transition-opacity group-hover:opacity-0">{lang}</span>}
        <button
          type="button"
          onClick={() => void copy()}
          aria-label={t("forum_copy_code")}
          className="inline-flex items-center gap-1 rounded border border-divider bg-surface px-1.5 py-0.5 opacity-0 transition-opacity hover:text-ink focus:opacity-100 group-hover:opacity-100"
        >
          {copied ? <Check size={12} /> : <Copy size={12} />}
          {copied ? t("forum_code_copied") : t("forum_copy_code")}
        </button>
      </div>
    </div>
  );
}
