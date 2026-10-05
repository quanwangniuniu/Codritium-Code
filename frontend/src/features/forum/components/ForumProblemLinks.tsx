import Link from "next/link";
import { MessagesSquare, SquarePen } from "lucide-react";
import { t } from "@/shared/i18n";

// Top of a problem's Discussion tab: the problem's own comments stay below,
// and these lead to longer-form forum posts about it.
export function ForumProblemLinks({ slug }: { slug: string }) {
  const q = encodeURIComponent(slug);
  return (
    <div className="mb-4 flex flex-wrap items-center gap-x-4 gap-y-2 rounded-md border border-divider bg-surface-2/60 px-3 py-2 text-sm">
      <Link href={`/forums?problem=${q}`} className="inline-flex items-center gap-1.5 text-ink hover:text-accent">
        <MessagesSquare size={15} />
        {t("forum_problem_posts_link")}
      </Link>
      <Link href={`/forums/new?problem=${q}`} className="ml-auto inline-flex items-center gap-1.5 text-accent hover:underline">
        <SquarePen size={14} />
        {t("forum_problem_write_post")}
      </Link>
    </div>
  );
}
