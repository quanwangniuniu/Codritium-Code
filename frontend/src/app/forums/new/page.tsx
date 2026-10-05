import { redirect } from "next/navigation";
import { currentUser } from "@/features/auth/server";
import { listProblems } from "@/features/problems/server";
import { t } from "@/shared/i18n";
import { FORUM_SECTIONS, type ForumSection } from "@/features/forum/api";
import { ForumPostEditor } from "@/features/forum/components/ForumPostEditor";

interface NewForumPostProps {
  searchParams: Promise<{ section?: string; problem?: string }>;
}

export default async function NewForumPostPage({ searchParams }: NewForumPostProps) {
  const { section, problem } = await searchParams;
  const user = await currentUser();
  if (!user) {
    const back = problem ? `/forums/new?problem=${encodeURIComponent(problem)}` : "/forums/new";
    redirect(`/login?next=${encodeURIComponent(back)}`);
  }
  const problems = await listProblems();
  const defaultProblem = problems.some((p) => p.id === problem) ? problem : undefined;

  return (
    <div className="mx-auto max-w-3xl space-y-6 px-4 py-6 sm:px-6 lg:py-8">
      <h1 className="text-xl font-semibold">{t("forum_new_post_title")}</h1>
      <ForumPostEditor
        defaultSection={FORUM_SECTIONS.includes(section as ForumSection) ? (section as ForumSection) : undefined}
        defaultProblem={defaultProblem}
        problems={problems.map((p) => ({ slug: p.id, title: p.title }))}
      />
    </div>
  );
}
