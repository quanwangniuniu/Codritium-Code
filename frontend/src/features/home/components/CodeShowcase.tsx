"use client";

import { useState } from "react";
import Link from "next/link";
import { Check, Code2, Copy, SquareArrowOutUpRight } from "lucide-react";
import { IDEPane } from "@/shared/editor/IDEPane";
import { useMonacoTheme } from "@/shared/editor/useMonacoTheme";
import { languageForFile } from "@/shared/editor/language";
import { t } from "@/shared/i18n";
import { cn } from "@/shared/lib/cn";
import { ArrowLink } from "./ArrowLink";

export interface ShowcaseProblem {
  slug: string;
  title: string;
  file: string;
  code: string;
}

export interface ShowcaseTrack {
  label: string;
  problems: ShowcaseProblem[];
}

// Read-only Monaco preview of real starter code, grouped by track. The side
// list switches problems; "Open in workspace" goes to the full problem page.
export function CodeShowcase({ tracks }: { tracks: ShowcaseTrack[] }) {
  const [trackIdx, setTrackIdx] = useState(0);
  const [problemIdx, setProblemIdx] = useState(0);
  const [copied, setCopied] = useState(false);
  const monacoTheme = useMonacoTheme();

  const track = tracks[trackIdx];
  const problem = track?.problems[problemIdx];
  if (!problem) return null;

  const selectTrack = (i: number) => {
    setTrackIdx(i);
    setProblemIdx(0);
  };

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(problem.code);
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch {
      // clipboard blocked (insecure context / permissions) — nothing to do
    }
  };

  return (
    <div className="grid lg:grid-cols-[1fr_260px] gap-6 items-start text-left">
      <div className="home-window overflow-hidden">
        <div className="flex flex-wrap items-center gap-2 border-b border-divider bg-surface-2/60 px-2 py-1.5">
          <div role="tablist" className="flex">
            {tracks.map((tr, i) => (
              <button
                key={tr.label}
                role="tab"
                aria-selected={i === trackIdx}
                onClick={() => selectTrack(i)}
                className={cn(
                  "px-3.5 py-2 text-sm rounded-lg transition-colors",
                  i === trackIdx ? "bg-surface text-ink font-medium shadow-sm" : "text-muted hover:text-ink",
                )}
              >
                {tr.label}
              </button>
            ))}
          </div>
          <div className="ml-auto flex items-center gap-2">
            <button
              type="button"
              onClick={copy}
              className="inline-flex items-center gap-1.5 rounded-lg border border-divider bg-surface px-3 py-1.5 text-sm text-muted hover:text-ink"
            >
              {copied ? <Check size={14} /> : <Copy size={14} />}
              {copied ? t("home_showcase_copied") : t("home_showcase_copy")}
            </button>
            <Link
              href={`/problems/${problem.slug}`}
              className="inline-flex items-center gap-1.5 rounded-lg bg-home-green px-3 py-1.5 text-sm font-medium text-white hover:opacity-90"
            >
              <SquareArrowOutUpRight size={14} />
              {t("home_showcase_open")}
            </Link>
          </div>
        </div>
        <div className="flex items-center gap-1.5 px-4 h-9 border-b border-divider text-xs text-muted mono">
          <Code2 size={13} />
          {problem.file}
        </div>
        <div className="h-[380px] sm:h-[440px] bg-surface">
          <IDEPane
            path={`showcase/${problem.slug}/${problem.file}`}
            language={languageForFile(problem.file)}
            value={problem.code}
            readOnly
            theme={monacoTheme}
          />
        </div>
      </div>

      <div>
        <ul className="space-y-1">
          {track.problems.map((p, i) => (
            <li key={p.slug}>
              <button
                type="button"
                onClick={() => setProblemIdx(i)}
                className={cn(
                  "w-full flex gap-2.5 rounded-xl px-3.5 py-3 text-left text-sm transition-colors",
                  i === problemIdx
                    ? "bg-surface border border-divider shadow-sm text-home-teal font-medium"
                    : "text-accent hover:bg-surface-2 border border-transparent",
                )}
              >
                <Code2 size={16} className="shrink-0 mt-0.5" />
                <span className="line-clamp-2">{p.title}</span>
              </button>
            </li>
          ))}
        </ul>
        <div className="mt-4 pt-4 border-t border-divider px-3.5">
          <ArrowLink href="/problems">{t("home_showcase_browse_all")}</ArrowLink>
        </div>
      </div>
    </div>
  );
}
