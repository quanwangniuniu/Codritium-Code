import Link from "next/link";
import { ArrowRight } from "lucide-react";
import { t } from "@/lib/i18n";
import { ProductMock } from "./ProductMock";

interface HomeHeroProps {
  signedIn: boolean;
  problemCount: number;
  trackCount: number;
}

export function HomeHero({ signedIn, problemCount, trackCount }: HomeHeroProps) {
  const stats = [
    { value: problemCount, label: t("home_stat_problems") },
    { value: trackCount, label: t("home_stat_tracks") },
    { value: 5, label: t("home_stat_dimensions") },
  ];

  return (
    <section className="relative overflow-hidden">
      <div className="home-hero-bg" />
      <div className="relative mx-auto max-w-[1200px] px-4 sm:px-6 pt-14 pb-24 sm:pb-52 lg:pt-20 lg:pb-44 grid lg:grid-cols-[1fr_1.1fr] gap-14 lg:gap-16 items-center">
        <div className="space-y-6">
          <p className="inline-flex items-center gap-2 rounded-full border border-divider bg-surface px-3 py-1 text-xs font-medium text-muted">
            <span className="size-1.5 rounded-full bg-home-green" />
            {t("home_hero_eyebrow")}
          </p>
          <h1 className="text-4xl sm:text-5xl lg:text-[3.4rem] font-semibold tracking-tight leading-[1.08]">
            {t("home_hero_title")}
          </h1>
          <p className="text-base sm:text-lg text-muted leading-relaxed max-w-xl">
            {t("home_hero_blurb")}
          </p>
          <div className="flex flex-wrap items-center gap-3 pt-1">
            {signedIn ? (
              <Link href="/problems" className="btn-pill btn-pill--primary btn-pill--lg">
                {t("home_cta_continue")}
                <ArrowRight size={16} />
              </Link>
            ) : (
              <>
                <Link href="/login?mode=register" className="btn-pill btn-pill--primary btn-pill--lg">
                  {t("home_cta_create_account")}
                  <ArrowRight size={16} />
                </Link>
                <Link href="/problems" className="btn-pill btn-pill--ghost btn-pill--lg">
                  {t("home_cta_browse")}
                </Link>
              </>
            )}
          </div>
          <dl className="flex flex-wrap gap-x-10 gap-y-4 pt-4">
            {stats.map((s) => (
              <div key={s.label} className="flex flex-col-reverse">
                <dt className="text-xs text-muted mt-0.5">{s.label}</dt>
                <dd className="text-2xl font-semibold tracking-tight">{s.value}</dd>
              </div>
            ))}
          </dl>
        </div>
        <ProductMock />
      </div>
    </section>
  );
}
