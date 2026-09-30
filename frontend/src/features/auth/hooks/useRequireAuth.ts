// Route gate — calls /api/me on mount and redirects to /login if the user is
// not authenticated. Returns the loaded Me so the caller can render once the
// check passes.
//
// Usage in any protected page component:
//
//   const me = useRequireAuth();
//   if (!me) return null;  // gate is still resolving / redirecting

"use client";

import { useEffect, useState } from "react";
import { usePathname, useRouter } from "next/navigation";
import { authApi } from "@/features/auth/api";
import type { Me } from "@/features/auth/types";

export function useRequireAuth(): Me | null {
  const router = useRouter();
  const pathname = usePathname();
  const [me, setMe] = useState<Me | null>(null);

  useEffect(() => {
    let cancelled = false;
    authApi.me()
      .then((u) => {
        if (!cancelled) setMe(u);
      })
      .catch(() => {
        if (cancelled) return;
        // Preserve where the user was going so /login can send them back.
        const next = pathname && pathname !== "/login" ? `?next=${encodeURIComponent(pathname)}` : "";
        router.replace(`/login${next}`);
      });
    return () => {
      cancelled = true;
    };
  }, [router, pathname]);

  return me;
}
