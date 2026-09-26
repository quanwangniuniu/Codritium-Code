import { Plus, Sparkles, MessagesSquare, Briefcase, Coins, Megaphone } from "lucide-react";
import { t, type LocaleKey } from "@/lib/i18n";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import type { ForumSection } from "@/lib/types";

const SECTIONS: {
  key: ForumSection;
  labelKey: LocaleKey;
  taglineKey: LocaleKey;
  icon: typeof Briefcase;
}[] = [
  {
    key: "interview",
    labelKey: "forum_section_interview_label",
    taglineKey: "forum_section_interview_tagline",
    icon: MessagesSquare,
  },
  {
    key: "career",
    labelKey: "forum_section_career_label",
    taglineKey: "forum_section_career_tagline",
    icon: Briefcase,
  },
  {
    key: "compensation",
    labelKey: "forum_section_compensation_label",
    taglineKey: "forum_section_compensation_tagline",
    icon: Coins,
  },
  {
    key: "feedback",
    labelKey: "forum_section_feedback_label",
    taglineKey: "forum_section_feedback_tagline",
    icon: Megaphone,
  },
];

// Community hub — placeholder for launch. Section skeleton is live; posting and
// the post feed wire up to the backend in a later pass.
export default function ForumsPage() {
  return (
    <div className="mx-auto max-w-[1600px] px-6 py-10 space-y-8">
      <header className="flex items-end justify-between flex-wrap gap-4">
        <div className="space-y-1">
          <h1 className="text-3xl tracking-tight font-semibold">{t("nav_forums")}</h1>
          <p className="text-sm text-muted max-w-2xl leading-relaxed">
            {t("forum_subtitle")}
          </p>
        </div>
        <button
          type="button"
          disabled
          className="btn-pill btn-pill--primary text-sm opacity-60 cursor-not-allowed"
        >
          <Plus size={15} />
          {t("forums_start_discussion")}
        </button>
      </header>

      <div className="flex items-center gap-3 rounded-xl border border-dashed border-divider-strong bg-surface px-5 py-4 text-sm text-muted">
        <Sparkles size={18} className="shrink-0 text-accent" />
        {t("forums_locked_note")}
      </div>

      <div className="grid gap-4 sm:grid-cols-2">
        {SECTIONS.map((s) => {
          const label = t(s.labelKey);
          return (
            <Card key={s.key} className="flex h-full flex-col">
              <CardHeader>
                <div className="flex items-start gap-3">
                  <span className="grid h-10 w-10 shrink-0 place-items-center rounded-lg bg-surface-2 text-muted">
                    <s.icon size={18} strokeWidth={1.8} />
                  </span>
                  <div className="min-w-0 flex-1">
                    <CardTitle className="text-base">{label}</CardTitle>
                    <CardDescription className="mt-0.5 leading-relaxed">
                      {t(s.taglineKey)}
                    </CardDescription>
                  </div>
                  <Badge tone="info">{t("forums_coming_soon")}</Badge>
                </div>
              </CardHeader>
              <CardContent className="mt-auto pt-0">
                <p className="text-xs text-faint">
                  {t("forum_no_posts_fmt", { params: { section: label } })}
                </p>
              </CardContent>
            </Card>
          );
        })}
      </div>
    </div>
  );
}
