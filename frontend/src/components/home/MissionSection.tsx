import { Compass } from "lucide-react";
import { t, type LocaleKey } from "@/shared/i18n";
import { cn } from "@/lib/utils";
import { ArrowLink } from "./ArrowLink";
import { CONTACT_EMAIL } from "./AudienceSection";
import { ACCENT_TEXT, SectionBadge } from "./SectionBadge";

const SKILLS: LocaleKey[] = [
  "home_skill_incidents",
  "home_skill_security",
  "home_skill_refactor",
  "home_skill_patches",
  "home_skill_tests",
  "home_skill_tradeoffs",
];

export function MissionSection() {
  return (
    <section className="mx-auto max-w-[1200px] px-4 sm:px-6 py-20 lg:py-24 text-center">
      <SectionBadge icon={Compass} accent="rose" />
      <h2 className={cn("mt-6 text-3xl sm:text-4xl font-semibold tracking-tight", ACCENT_TEXT.rose)}>
        {t("home_mission_title")}
      </h2>
      <p className="mt-5 max-w-3xl mx-auto text-muted leading-relaxed sm:text-lg">{t("home_mission_body")}</p>
      <p className="mt-12 text-xs uppercase tracking-wider text-faint">{t("home_mission_skills_label")}</p>
      <ul className="mt-5 flex flex-wrap justify-center gap-3 max-w-3xl mx-auto">
        {SKILLS.map((k) => (
          <li
            key={k}
            className="rounded-full border border-divider bg-surface px-4 py-2 text-sm text-muted"
          >
            {t(k)}
          </li>
        ))}
      </ul>

      <div className="mt-20 pt-16 border-t border-divider max-w-3xl mx-auto">
        <h3 className="text-xl sm:text-2xl font-semibold tracking-tight">{t("home_hiring_title")}</h3>
        <p className="mt-3 text-muted leading-relaxed">{t("home_hiring_body")}</p>
        <div className="mt-5 flex justify-center">
          <ArrowLink href={`mailto:${CONTACT_EMAIL}`}>{t("home_hiring_link")}</ArrowLink>
        </div>
      </div>
    </section>
  );
}
