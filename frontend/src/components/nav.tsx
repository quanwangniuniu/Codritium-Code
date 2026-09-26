import Link from "next/link";
import { Bell, LogIn } from "lucide-react";
import { currentUser } from "@/lib/auth";
import { t } from "@/lib/i18n";
import { CodritiumLogo } from "@/components/CodritiumLogo";
import { ThemeToggle } from "@/components/ThemeToggle";
import { NavUserMenu } from "@/components/NavUserMenu";

export async function Nav() {
  const user = await currentUser();
  return (
    <header className="sticky top-0 z-30 border-b border-divider bg-canvas/80 backdrop-blur">
      <div className="mx-auto max-w-[1600px] px-6 h-14 flex items-center justify-between">
        <div className="flex items-center gap-5">
          <Link href="/" className="flex items-center" aria-label="Codritium">
            <CodritiumLogo height={44} />
          </Link>
          <nav className="hidden sm:flex items-center gap-1 text-sm">
            <Link href="/problems" className="nav-pill">
              {t("nav_problems")}
            </Link>
            <Link href="/forums" className="nav-pill">
              {t("nav_forums")}
            </Link>
            {user && (
              <Link href="/dashboard" className="nav-pill">
                {t("nav_dashboard")}
              </Link>
            )}
          </nav>
        </div>
        <div className="flex items-center gap-1.5">
          {user ? (
            <>
              <button
                type="button"
                aria-label={t("nav_notifications")}
                className="nav-icon-btn"
              >
                <Bell size={18} strokeWidth={1.8} />
              </button>
              <ThemeToggle />
              <NavUserMenu
                displayName={user.display_name}
                avatarUrl={user.avatar_url || undefined}
              />
            </>
          ) : (
            <>
              <ThemeToggle />
              <Link href="/login" className="btn-pill btn-pill--primary text-sm">
                <LogIn size={14} />
                {t("sign_in")}
              </Link>
            </>
          )}
        </div>
      </div>
    </header>
  );
}
