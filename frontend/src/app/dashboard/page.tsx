import { redirect } from "next/navigation";

// The dashboard merged into the LeetCode-style profile page.
export default function DashboardPage() {
  redirect("/profile");
}
