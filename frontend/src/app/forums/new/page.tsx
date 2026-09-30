import { redirect } from "next/navigation";
import { currentUser } from "@/features/auth/server";
import { listProblems } from "@/features/problems/server";
import { t } from "@/shared/i18n";
import { FORUM_SECTIONS, type ForumSection } from "@/features/forum/api";
import { ForumPostEditor } from "@/features/forum/components/ForumPostEditor";

interface NewForumPostProps {
  searchParams: Promise<{ section?: string }>;
}

export default async function NewForumPostPage({ searchParams }: NewForumPostProps) {
  const user = await currentUser();
  if (!user) redirect("/login?next=%2Fforums%2Fnew");
  const { section } = await searchParams;
  const problems = await listProblems();

  return (
    <div className="mx-auto max-w-3xl space-y-6 px-4 py-6 sm:px-6 lg:py-8">
      <h1 className="text-xl font-semibold">{t("forum_new_post_title")}</h1>
      <ForumPostEditor
        defaultSection={FORUM_SECTIONS.includes(section as ForumSection) ? (section as ForumSection) : undefined}
        problems={problems.map((p) => ({ slug: p.id, title: p.title }))}
      />
    </div>
  );
}
