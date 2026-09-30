import { notFound, redirect } from "next/navigation";
import { currentUser } from "@/lib/auth";
import { listProblems } from "@/lib/store";
import { t } from "@/shared/i18n";
import { getForumPost } from "@/lib/forum-server";
import { ForumPostEditor } from "@/components/forum/ForumPostEditor";

interface EditForumPostProps {
  params: Promise<{ id: string }>;
}

export default async function EditForumPostPage({ params }: EditForumPostProps) {
  const { id } = await params;
  const user = await currentUser();
  if (!user) redirect(`/login?next=${encodeURIComponent(`/forums/${id}/edit`)}`);
  const [post, problems] = await Promise.all([getForumPost(id), listProblems()]);
  // Only the author can edit; the API enforces this too.
  if (!post || !post.is_mine) notFound();

  return (
    <div className="mx-auto max-w-3xl space-y-6 px-4 py-6 sm:px-6 lg:py-8">
      <h1 className="text-xl font-semibold">{t("forum_edit_post_title")}</h1>
      <ForumPostEditor
        postId={post.id}
        initial={{
          section: post.section,
          title: post.title,
          body_md: post.body_md ?? "",
          tags: post.tags,
          is_anonymous: post.is_anonymous,
          problem_slug: post.problem_slug,
        }}
        problems={problems.map((p) => ({ slug: p.id, title: p.title }))}
      />
    </div>
  );
}
