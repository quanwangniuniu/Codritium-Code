import { notFound, redirect } from "next/navigation";
import { currentUser } from "@/features/auth/server";
import { ForumModerationPage } from "@/features/forum/components/ForumModerationPage";

// Moderators only; everyone else gets a plain 404 rather than a hint the
// page exists. The API enforces the same check.
export default async function Page() {
  const user = await currentUser();
  if (!user) redirect("/login?next=%2Fadmin%2Fforum");
  if (user.role !== "admin") notFound();
  return <ForumModerationPage />;
}
