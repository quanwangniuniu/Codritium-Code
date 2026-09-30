import { redirect } from "next/navigation";
import { ActivityCard } from "@/components/profile/ActivityCard";
import { BadgesCard } from "@/components/profile/BadgesCard";
import { ProfileSidebar } from "@/components/profile/ProfileSidebar";
import { PROFILE_TABS, ProfileTabs, type ProfileTab } from "@/components/profile/ProfileTabs";
import { SolvedCard } from "@/components/profile/SolvedCard";
import { currentUser } from "@/lib/auth";
import { getMyProfile, listProblems, listUserSubmissions } from "@/lib/store";

interface ProfilePageProps {
  searchParams: Promise<{ tab?: string }>;
}

// LeetCode-style profile: identity + community in the sidebar; solved
// progress, badges, the submission calendar, and activity tabs on the right.
// This page absorbed the old /dashboard.
export default async function ProfilePage({ searchParams }: ProfilePageProps) {
  const user = await currentUser();
  if (!user) redirect("/login");
  const { tab: rawTab } = await searchParams;
  const tab: ProfileTab = PROFILE_TABS.some((tb) => tb.id === rawTab) ? (rawTab as ProfileTab) : "recent";

  const profile = await getMyProfile();
  if (!profile) redirect("/login");

  // The full, deletable submission history is only needed on its own tab.
  const [allSubmissions, problems] =
    tab === "all" ? await Promise.all([listUserSubmissions(user.id), listProblems()]) : [undefined, []];
  const problemMap = Object.fromEntries(problems.map((p) => [p.id, p]));

  return (
    <div className="mx-auto max-w-[1280px] px-4 sm:px-6 py-6 sm:py-8">
      <div className="grid lg:grid-cols-[300px_1fr] gap-5">
        <ProfileSidebar profile={profile} />
        <div className="min-w-0 space-y-5">
          <div className="grid md:grid-cols-[1.35fr_1fr] gap-5">
            <SolvedCard solved={profile.solved} />
            <BadgesCard profile={profile} />
          </div>
          <ActivityCard calendar={profile.calendar} />
          <ProfileTabs tab={tab} profile={profile} allSubmissions={allSubmissions} problemMap={problemMap} />
        </div>
      </div>
    </div>
  );
}
