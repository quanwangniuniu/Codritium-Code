"use client";

import { useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { ChevronDown, UserRound, Settings, CreditCard, LogOut } from "lucide-react";
import { authApi } from "@/features/auth/api";
import { UserAvatar } from "@/shared/avatar/UserAvatar";
import { t } from "@/shared/i18n";
import { useLocale } from "@/shared/i18n/client";

interface NavUserMenuProps {
  displayName: string;
  handle: string;
  avatarUrl?: string;
}

// Avatar + dropdown for the website nav (Profile / Settings / Plans / Logout).
// Client component: the dropdown is interactive and the logout call runs in the
// browser so the session cookie clears on the backend before we redirect.
export function NavUserMenu({ displayName, handle, avatarUrl }: NavUserMenuProps) {
  useLocale();
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);
  const router = useRouter();

  useEffect(() => {
    if (!open) return;
    function onDown(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false);
    }
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") setOpen(false);
    }
    document.addEventListener("mousedown", onDown);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("mousedown", onDown);
      document.removeEventListener("keydown", onKey);
    };
  }, [open]);

  async function logout() {
    setOpen(false);
    await authApi.logout().catch(() => null);
    router.replace("/");
    router.refresh();
  }

  const items = [
    { label: t("nav_profile"), href: "/profile", icon: UserRound },
    { label: t("settings_link"), href: "/settings", icon: Settings },
    { label: t("menu_plans"), href: "/plans", icon: CreditCard },
  ];

  return (
    <div ref={ref} className="relative">
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        aria-haspopup="menu"
        aria-expanded={open}
        aria-label={displayName}
        className="flex items-center gap-1 rounded-full pl-0.5 pr-1.5 py-0.5 transition-colors hover:bg-surface-2"
      >
        <UserAvatar url={avatarUrl} seed={handle} size={28} />
        <ChevronDown
          size={14}
          strokeWidth={2}
          className={"text-muted transition-transform " + (open ? "rotate-180" : "")}
        />
      </button>

      {open && (
        <div
          role="menu"
          className="absolute right-0 top-full z-50 mt-2 w-44 overflow-hidden rounded-xl border border-divider bg-surface py-1 shadow-lg"
        >
          {items.map((it) => (
            <Link
              key={it.href}
              href={it.href}
              role="menuitem"
              onClick={() => setOpen(false)}
              className="flex items-center gap-2.5 px-3 py-2 text-sm text-ink transition-colors hover:bg-surface-2"
            >
              <it.icon size={15} strokeWidth={1.8} className="text-muted" />
              {it.label}
            </Link>
          ))}
          <div className="my-1 border-t border-divider" />
          <button
            type="button"
            role="menuitem"
            onClick={logout}
            className="flex w-full items-center gap-2.5 px-3 py-2 text-sm text-ink transition-colors hover:bg-surface-2"
          >
            <LogOut size={15} strokeWidth={1.8} className="text-muted" />
            {t("sign_out")}
          </button>
        </div>
      )}
    </div>
  );
}
