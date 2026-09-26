import Link from "next/link";
import { Lightbulb, ShieldAlert, ChevronUp, MessageCircle, Pin } from "lucide-react";
import { Markdown } from "@/components/markdown";
import { Badge } from "@/components/ui/badge";
import { UserLink } from "@/components/user-link";
import { cn } from "@/lib/utils";
import { t, type LocaleKey } from "@/lib/i18n";
import type { Problem, Discussion, PublicSolution, User } from "@/lib/types";

export const PROBLEM_TABS: { value: "brief" | "hint" | "anti-patterns" | "discussion" | "solutions"; labelKey: LocaleKey }[] = [
  { value: "brief", labelKey: "problem_tab_brief" },
  { value: "hint", labelKey: "problem_tab_hint" },
  { value: "anti-patterns", labelKey: "problem_tab_anti_patterns" },
  { value: "discussion", labelKey: "problem_tab_discussion" },
  { value: "solutions", labelKey: "problem_tab_solutions" },
];

export type ProblemTabValue = (typeof PROBLEM_TABS)[number]["value"];

const ANTI_PATTERN_GUIDE: { nameKey: LocaleKey; descKey: LocaleKey }[] = [
  { nameKey: "antipattern_hands_off_name", descKey: "antipattern_hands_off_desc" },
  { nameKey: "antipattern_feature_marathon_name", descKey: "antipattern_feature_marathon_desc" },
  { nameKey: "antipattern_ai_showcase_name", descKey: "antipattern_ai_showcase_desc" },
  { nameKey: "antipattern_not_thinking_name", descKey: "antipattern_not_thinking_desc" },
];

interface ProblemTabsProps {
  problem: Problem;
  activeTab: ProblemTabValue;
  discussions: Discussion[];
  solutions: PublicSolution[];
  userMap: Map<string, User>;
}

export function ProblemTabs({
  problem,
  activeTab,
  discussions,
  solutions,
  userMap,
}: ProblemTabsProps) {
  return (
    <div className="space-y-3 h-full flex flex-col min-h-0">
      <div className="flex items-center gap-1 border-b border-divider flex-shrink-0">
        {PROBLEM_TABS.map((tab) => {
          const active = activeTab === tab.value;
          return (
            <Link
              key={tab.value}
              href={`/problems/${problem.id}${tab.value === "brief" ? "" : `?tab=${tab.value}`}`}
              className={cn(
                "px-3 py-2 text-sm border-b-2 -mb-px transition-colors",
                active
                  ? "border-accent text-ink"
                  : "border-transparent text-muted hover:text-ink",
              )}
            >
              {t(tab.labelKey)}
            </Link>
          );
        })}
      </div>

      <div className="flex-1 min-h-0 overflow-y-auto pr-1">
        {activeTab === "brief" && (
          <div className="space-y-4">
            {problem.canva_fuzzification_prefix_md && (
              <div className="rounded-lg border border-warning/30 bg-warning-soft p-4">
                <div className="text-xs font-medium text-warning mb-1">
                  {t("fuzzy_spec_label")}
                </div>
                <Markdown source={problem.canva_fuzzification_prefix_md} />
              </div>
            )}
            <div className="rounded-lg border border-divider bg-surface p-4">
              <Markdown source={problem.description_md} />
            </div>
          </div>
        )}

        {activeTab === "hint" && (
          <div className="rounded-lg border border-divider bg-surface p-4">
            {problem.hint ? (
              <div className="flex gap-3 text-sm">
                <Lightbulb size={16} className="mt-0.5 flex-shrink-0 text-warning" />
                <div>
                  <div className="font-medium mb-1 text-ink">{t("hint_heading")}</div>
                  <p className="text-muted leading-relaxed">{problem.hint}</p>
                </div>
              </div>
            ) : (
              <p className="text-sm text-muted">{t("no_hint_placeholder")}</p>
            )}
          </div>
        )}

        {activeTab === "anti-patterns" && (
          <div className="rounded-lg border border-divider bg-surface p-4 space-y-3">
            <div className="flex items-center gap-2 text-sm text-ink">
              <ShieldAlert size={16} className="text-danger" />
              <span className="font-medium">{t("four_anti_patterns_grader")}</span>
            </div>
            <ul className="space-y-2.5">
              {ANTI_PATTERN_GUIDE.map((p) => (
                <li key={p.nameKey} className="text-sm">
                  <div className="font-medium text-ink">{t(p.nameKey)}</div>
                  <p className="text-muted leading-relaxed">{t(p.descKey)}</p>
                </li>
              ))}
            </ul>
            <p className="text-xs text-faint pt-2 border-t border-divider">
              {t("anti_pattern_deduction_note")}
            </p>
          </div>
        )}

        {activeTab === "discussion" && (
          <div className="space-y-3">
            {discussions.length === 0 ? (
              <p className="rounded-lg border border-divider bg-surface p-4 text-sm text-muted">
                {t("no_problem_discussion_prefix")}{" "}
                <Link href="/forum" className="underline text-ink">
                  {t("forum_word")}
                </Link>
                .
              </p>
            ) : (
              discussions.map((d) => {
                const author = userMap.get(d.author_id);
                return (
                  <div
                    key={d.id}
                    className="rounded-lg border border-divider bg-surface p-3 transition-colors hover:border-divider-strong"
                  >
                    <div className="flex items-start gap-3">
                      <div className="flex flex-col items-center pt-0.5 w-8 flex-shrink-0">
                        {d.is_pinned && <Pin size={10} className="text-warning mb-0.5" />}
                        <ChevronUp size={12} className="text-faint" />
                        <span className="text-xs font-semibold text-ink tabular-nums">
                          {d.upvote_count}
                        </span>
                      </div>
                      <div className="flex-1 min-w-0">
                        <Link
                          href={`/forum/post/${d.id}`}
                          className="text-sm font-medium text-ink truncate hover:underline block"
                        >
                          {d.title}
                        </Link>
                        <p className="text-xs text-muted line-clamp-2 mt-0.5">
                          {d.body_md.slice(0, 140).replace(/[*#`>]/g, "")}
                        </p>
                        <div className="flex flex-wrap items-center gap-x-3 gap-y-1 mt-1.5 text-[11px]">
                          {author && <UserLink user={author} />}
                          <Link
                            href={`/forum/post/${d.id}`}
                            className="inline-flex items-center gap-0.5 text-faint hover:text-ink"
                          >
                            <MessageCircle size={10} /> {d.reply_count}
                          </Link>
                          {d.tags.slice(0, 3).map((tg) => (
                            <Badge key={tg} tone="neutral">
                              {tg}
                            </Badge>
                          ))}
                        </div>
                      </div>
                    </div>
                  </div>
                );
              })
            )}
            <p className="text-xs text-faint pt-1">
              {t("per_problem_discussion_note_prefix")}{" "}
              <Link href="/forum" className="underline text-ink">
                {t("global_forum_word")}
              </Link>
              .
            </p>
          </div>
        )}

        {activeTab === "solutions" && (
          <div className="space-y-3">
            {solutions.length === 0 ? (
              <p className="rounded-lg border border-divider bg-surface p-4 text-sm text-muted">
                {t("no_published_solutions")}
              </p>
            ) : (
              solutions.map((s) => {
                const author = userMap.get(s.author_id);
                return (
                  <div key={s.id} className="rounded-lg border border-divider bg-surface p-4">
                    <div className="flex items-start gap-4">
                      <div className="flex flex-col items-center w-12 flex-shrink-0">
                        <span className="text-lg font-semibold text-ink tabular-nums">
                          {s.used_by_count.toLocaleString()}
                        </span>
                        <span className="text-[10px] text-faint uppercase tracking-wider">
                          {t("used_by_label")}
                        </span>
                      </div>
                      <div className="flex-1 min-w-0">
                        <div className="flex items-start justify-between gap-3">
                          <div className="text-sm font-medium text-ink">{s.title}</div>
                          <Badge tone="info">{s.approach_label}</Badge>
                        </div>
                        <p className="text-xs text-muted leading-relaxed mt-1.5">{s.writeup_md}</p>
                        <div className="flex flex-wrap items-center gap-x-3 gap-y-1 mt-2 text-[11px]">
                          {author && <UserLink user={author} />}
                          <span className="text-faint">
                            {new Date(s.created_at).toLocaleDateString()}
                          </span>
                          <span className="inline-flex items-center gap-0.5 text-faint">
                            <ChevronUp size={10} /> {s.upvote_count}
                          </span>
                        </div>
                      </div>
                    </div>
                  </div>
                );
              })
            )}
            <p className="text-xs text-faint pt-1">{t("solutions_sort_note")}</p>
          </div>
        )}
      </div>
    </div>
  );
}
