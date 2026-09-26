import Link from "next/link";
import { redirect } from "next/navigation";
import { currentUser } from "@/lib/auth";
import { t, type LocaleKey } from "@/lib/i18n";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";

// Profile = identity + account. Practice activity (heatmap, scores, submission
// history) lives on /dashboard; pricing tiers live on /plans.
export default async function ProfilePage() {
  const user = await currentUser();
  if (!user) redirect("/login");

  const initial = (user.display_name || "?").trim().charAt(0).toUpperCase();
  const tierKey = ("tier_" + (user.tier ?? "standard")) as LocaleKey;

  return (
    <div className="mx-auto max-w-3xl px-6 py-10 space-y-8">
      <header className="flex items-center gap-5">
        <span className="grid h-20 w-20 place-items-center overflow-hidden rounded-full bg-surface-2 text-2xl font-semibold text-muted">
          {user.avatar_url ? (
            // eslint-disable-next-line @next/next/no-img-element
            <img src={user.avatar_url} alt="" className="h-full w-full object-cover" />
          ) : (
            initial
          )}
        </span>
        <div className="min-w-0">
          <h1 className="text-3xl tracking-tight font-semibold truncate">
            {user.display_name}
          </h1>
          <p className="text-sm text-muted mt-1">
            {user.github_handle ? `@${user.github_handle}` : null}
            {user.region ? ` · ${user.region.toUpperCase()}` : null}
            {" · "}
            {user.is_pro ? t("tier_pro") : t("tier_free")}
          </p>
        </div>
      </header>

      <Card>
        <CardHeader className="pb-2">
          <CardDescription className="text-xs uppercase tracking-wider">
            {t("profile_bio_label")}
          </CardDescription>
        </CardHeader>
        <CardContent className="pt-0">
          <p className="text-sm text-muted leading-relaxed">{t("profile_bio_empty")}</p>
        </CardContent>
      </Card>

      <div className="grid sm:grid-cols-3 gap-3">
        <Card>
          <CardHeader className="pb-2">
            <CardDescription className="text-xs uppercase tracking-wider">
              {t("tier_label")}
            </CardDescription>
            <CardTitle className="text-2xl capitalize">{t(tierKey)}</CardTitle>
          </CardHeader>
          <CardContent className="pt-0">
            <Link
              href="/plans"
              className="text-xs text-accent hover:underline underline-offset-2"
            >
              {t("profile_view_plans_btn")}
            </Link>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription className="text-xs uppercase tracking-wider">
              {t("credits_label")}
            </CardDescription>
            <CardTitle className="text-2xl tabular-nums">{user.credits ?? 0}</CardTitle>
          </CardHeader>
          <CardContent className="pt-0">
            <p className="text-xs text-muted">{t("credits_placeholder_desc")}</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription className="text-xs uppercase tracking-wider">
              {t("badge_early_access_title")}
            </CardDescription>
            <CardTitle className="text-base">{t("badge_early_access_desc")}</CardTitle>
          </CardHeader>
        </Card>
      </div>
    </div>
  );
}
