import Link from "next/link";
import type { ReactNode } from "react";
import { ChevronRight } from "lucide-react";

// Text link with a trailing chevron, used for section calls to action.
export function ArrowLink({ href, children }: { href: string; children: ReactNode }) {
  return (
    <Link
      href={href}
      className="inline-flex items-center gap-1 text-sm font-medium text-accent hover:underline underline-offset-4"
    >
      {children}
      <ChevronRight size={16} />
    </Link>
  );
}
