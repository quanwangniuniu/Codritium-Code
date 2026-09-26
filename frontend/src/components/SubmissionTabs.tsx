"use client";

import Link from "next/link";
import { t } from "@/lib/i18n";

// SubmissionTabs sits at the top of /submissions/[id] and
// /submissions/[id]/reply and gives the candidate a single switch
// between the score Report and the Reply walkthrough. Each tab is a
// real link so the underlying Server Component pages keep their own
// data fetches.
export function SubmissionTabs({
  submissionId,
  active,
}: {
  submissionId: string;
  active: "reply" | "report";
}) {
  return (
    <div className="flex items-center gap-1 border-b border-divider mb-4">
      <TabLink
        href={`/submissions/${submissionId}/reply`}
        active={active === "reply"}
        label={t("reply_tab_label")}
      />
      <TabLink
        href={`/submissions/${submissionId}`}
        active={active === "report"}
        label={t("report_tab_label")}
      />
    </div>
  );
}

function TabLink({
  href,
  active,
  label,
}: {
  href: string;
  active: boolean;
  label: string;
}) {
  return (
    <Link
      href={href}
      aria-current={active ? "page" : undefined}
      className={
        "px-4 py-2 text-sm font-medium border-b-2 -mb-px transition-colors " +
        (active
          ? "border-accent text-ink"
          : "border-transparent text-muted hover:text-ink hover:border-divider-strong")
      }
    >
      {label}
    </Link>
  );
}
