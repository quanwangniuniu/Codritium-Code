import { redirect } from "next/navigation";
import type { Me } from "@/features/auth/types";
import { apiJSON } from "@/shared/api/server";
import { ProfileEditForm } from "./ProfileEditForm";

// /profile/edit: the signed-in user's editable profile. Loads the raw
// /api/me payload (the adapted User type drops the fields edited here).
export async function ProfileEditPage() {
  const me = await apiJSON<Me>("/api/me");
  if (!me) redirect("/login?next=/profile/edit");
  return <ProfileEditForm initial={me} />;
}
