import { redirect } from "next/navigation";
import { currentUser } from "@/features/auth/server";
import { NotificationsPage } from "@/features/notifications/components/NotificationsPage";

export default async function Page() {
  if (!(await currentUser())) redirect("/login?next=%2Fnotifications");
  return <NotificationsPage />;
}
