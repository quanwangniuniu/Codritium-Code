import Link from "next/link";
import { LogIn } from "lucide-react";
import { currentUser } from "@/features/auth/server";
import { NotificationBell } from "@/features/notifications/components/NotificationBell";
import { t } from "@/shared/i18n";
import { CodritiumLogo } from "@/shared/layout/CodritiumLogo";
import { ThemeToggle } from "@/shared/layout/ThemeToggle";
import { NavUserMenu } from "@/shared/layout/NavUserMenu";

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
              <Link href="/profile" className="nav-pill">
                {t("nav_profile")}
              </Link>
            )}
          </nav>
        </div>
        <div className="flex items-center gap-1.5">
          {user ? (
            <>
              <NotificationBell />
              <ThemeToggle />
              <NavUserMenu
                displayName={user.display_name}
                handle={user.handle}
                avatarUrl={user.avatar_url || undefined}
                isAdmin={user.role === "admin"}
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
