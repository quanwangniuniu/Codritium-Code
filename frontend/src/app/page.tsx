import Link from "next/link";
import {
  ArrowRight,
  BarChart3,
  BookOpen,
  Building2,
  Compass,
  MessageSquare,
  MessagesSquare,
} from "lucide-react";
import { DailyChallengeCard } from "@/components/daily-challenge-card";
import { Badge } from "@/components/ui/badge";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { getDailyChallenge, getProblem, listCompanies } from "@/lib/store";
import { currentUser } from "@/lib/auth";
import { t } from "@/lib/i18n";

const FEATURES = [
  { icon: MessageSquare, titleKey: "home_feature_chat_v2_title", descKey: "home_feature_chat_v2_desc" },
  { icon: Compass, titleKey: "home_feature_tips_agent_title", descKey: "home_feature_tips_agent_desc" },
  { icon: BarChart3, titleKey: "home_feature_five_dim_title", descKey: "home_feature_five_dim_desc" },
  { icon: Building2, titleKey: "home_feature_company_premium_title", descKey: "home_feature_company_premium_desc" },
  { icon: BookOpen, titleKey: "home_feature_reply_walkthrough_title", descKey: "home_feature_reply_walkthrough_desc" },
  { icon: MessagesSquare, titleKey: "home_feature_community_title", descKey: "home_feature_community_desc" },
] as const;

export default async function Home() {
  const user = await currentUser();
  const daily = await getDailyChallenge();
  const dailyProblem = daily ? await getProblem(daily.problem_id) : null;
  const companies = await listCompanies();

  return (
    <div className="relative">
      <div className="gradient-violet gradient-lime absolute inset-x-0 top-0 h-[480px] pointer-events-none" />
      <div className="relative mx-auto max-w-[1600px] px-6 py-10 space-y-16">
        <section className="grid lg:grid-cols-2 gap-10 items-center">
          <div className="space-y-5">
            <Badge tone="info">{t("home_hero_a_badge")}</Badge>
            <h1 className="text-4xl sm:text-5xl tracking-tight leading-tight font-semibold">
              {t("home_hero_a_title")}
            </h1>
            <p className="text-muted leading-relaxed max-w-xl">{t("home_hero_a_blurb")}</p>
            <div className="flex flex-wrap items-center gap-3">
              {user ? (
                <Link href="/problems" className="btn-pill btn-pill--primary btn-pill--lg">
                  {t("home_browse_problems")}
                  <ArrowRight size={16} />
                </Link>
              ) : (
                <Link href="/login" className="btn-pill btn-pill--primary btn-pill--lg">
                  {t("home_get_started")}
                </Link>
              )}
              {daily && (
                <Link
                  href={`/problems/${daily.problem_id}`}
                  className="btn-pill btn-pill--ghost btn-pill--lg"
                >
                  {t("home_try_today")}
                </Link>
              )}
            </div>
          </div>
          <div>
            {daily && dailyProblem && (
              <DailyChallengeCard problem={dailyProblem} date={daily.challenge_date} />
            )}
          </div>
        </section>

        <section className="grid lg:grid-cols-2 gap-10 items-center pt-4 border-t border-divider">
          <div className="order-2 lg:order-1 text-sm leading-relaxed text-muted space-y-3">
            <p className="text-faint uppercase tracking-wider text-xs">{t("home_lc_hr_label")}</p>
            <p>{t("home_hero_b_blurb")}</p>
          </div>
          <div className="order-1 lg:order-2 space-y-5">
            <Badge tone="success">{t("home_hero_b_badge")}</Badge>
            <h2 className="text-3xl sm:text-4xl tracking-tight leading-tight font-semibold">
              {t("home_hero_b_title")}
            </h2>
          </div>
        </section>

        <section className="space-y-6">
          <div>
            <h2 className="text-2xl tracking-tight font-semibold">
              {t("home_section_dims_title")}
            </h2>
            <p className="text-sm text-muted mt-1">{t("home_section_dims_blurb")}</p>
          </div>
          <div className="grid sm:grid-cols-2 lg:grid-cols-3 gap-4">
            {FEATURES.map((f) => (
              <Card key={f.titleKey}>
                <CardHeader>
                  <f.icon size={18} className="text-faint mb-2" />
                  <CardTitle className="text-base">{t(f.titleKey)}</CardTitle>
                  <CardDescription className="leading-relaxed mt-1">{t(f.descKey)}</CardDescription>
                </CardHeader>
                <CardContent />
              </Card>
            ))}
          </div>
        </section>

        <section className="pt-8 border-t border-divider space-y-4">
          <p className="text-xs uppercase tracking-wider text-faint">
            {t("home_b2b_logos_label")}
          </p>
          <div className="flex flex-wrap gap-3 items-center">
            {companies.map((c) => (
              <span
                key={c.slug}
                className="px-3 py-1.5 text-xs rounded-full border border-divider text-muted bg-card/50"
              >
                {c.name}
              </span>
            ))}
          </div>
          <p className="text-xs text-muted">{t("home_b2b_contact_hint")}</p>
        </section>
      </div>
    </div>
  );
}
