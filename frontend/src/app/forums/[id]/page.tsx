import type { Metadata } from "next";
import { getForumPost } from "@/features/forum/server";

export { ForumPostPage as default } from "@/features/forum/components/ForumPostPage";

// Link previews (Slack, X, iMessage…): the post's title and opening text.
// The author is left out so anonymous posts stay anonymous.
export async function generateMetadata({ params }: { params: Promise<{ id: string }> }): Promise<Metadata> {
  const { id } = await params;
  const post = await getForumPost(id);
  if (!post) return {};
  const description = post.excerpt || undefined;
  return {
    title: `${post.title} · Codritium Forums`,
    description,
    openGraph: { type: "article", title: post.title, description, siteName: "Codritium" },
    twitter: { card: "summary", title: post.title, description },
  };
}
