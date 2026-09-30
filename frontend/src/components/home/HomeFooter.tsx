import Link from "next/link";
import { t } from "@/lib/i18n";
import { CONTACT_EMAIL } from "./AudienceSection";

export function HomeFooter() {
  const links = [
    { href: "/problems", label: t("nav_problems") },
    { href: "/forums", label: t("nav_forums") },
    { href: "/plans", label: t("menu_plans") },
    { href: `mailto:${CONTACT_EMAIL}`, label: t("home_footer_contact") },
  ];
  return (
    <footer className="border-t border-divider">
      <div className="mx-auto max-w-[1200px] px-4 sm:px-6 py-8 flex flex-col sm:flex-row gap-4 items-center justify-between text-sm text-muted">
        <p>{t("home_footer_copyright", { params: { year: new Date().getFullYear() } })}</p>
        <nav className="flex flex-wrap justify-center items-center gap-x-5 gap-y-2">
          {links.map((l) => (
            <Link key={l.href} href={l.href} className="hover:text-ink">
              {l.label}
            </Link>
          ))}
        </nav>
      </div>
    </footer>
  );
}
