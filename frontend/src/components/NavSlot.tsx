"use client";

import { usePathname } from "next/navigation";

// Hides the demo2 chrome (top Nav) on routes that render their own full-screen
// shell — i.e. the IDE workspace. Without this gate the workspace would show
// the user identity twice (once in the demo2 Nav, once in the Codritium AppBar).
export function NavSlot({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  if (pathname && pathname.includes("/workspace")) return null;
  return <>{children}</>;
}
