import { Check } from "lucide-react";
import { t } from "@/shared/i18n";
import { ArrowLink } from "./ArrowLink";
import { DotGlyph } from "./DotGlyph";

export const CONTACT_EMAIL = "contact@codritium.com";

interface AudienceSectionProps {
  problemCount: number;
}

export function AudienceSection({ problemCount }: AudienceSectionProps) {
  return (
    <section className="border-y border-divider bg-surface">
      <div className="mx-auto max-w-[1200px] px-4 sm:px-6 py-20 grid md:grid-cols-2 md:divide-x divide-divider gap-y-16">
        <div className="md:pr-14 space-y-4">
          <DotGlyph name="community" />
          <h2 className="text-2xl sm:text-[1.75rem] font-semibold tracking-tight">{t("home_candidates_title")}</h2>
          <p className="text-muted leading-relaxed">
            {t("home_candidates_desc", { params: { n: problemCount } })}
          </p>
          <div className="flex flex-wrap gap-x-6 gap-y-2 pt-1">
            <ArrowLink href="/problems">{t("home_candidates_link_problems")}</ArrowLink>
            <ArrowLink href="/forums">{t("home_candidates_link_forums")}</ArrowLink>
          </div>
        </div>

        <div className="md:pl-14 space-y-4">
          <DotGlyph name="hiring" />
          <h2 className="text-2xl sm:text-[1.75rem] font-semibold tracking-tight">{t("home_teams_title")}</h2>
          <p className="text-muted leading-relaxed">{t("home_teams_desc")}</p>
          <ul className="space-y-2 text-sm text-muted">
            {(["home_teams_point_rubric", "home_teams_point_replay", "home_teams_point_problems"] as const).map(
              (k) => (
                <li key={k} className="flex gap-2">
                  <Check size={16} className="text-accent shrink-0 mt-0.5" />
                  {t(k)}
                </li>
              ),
            )}
          </ul>
          <div className="pt-1">
            <ArrowLink href={`mailto:${CONTACT_EMAIL}`}>{t("home_teams_link")}</ArrowLink>
          </div>
        </div>
      </div>
    </section>
  );
}
