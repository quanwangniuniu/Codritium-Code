"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { Play, RotateCcw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Backend } from "@/lib/api";
import { formatDateTime, formatRelativeTime } from "@/shared/format";
import { deleteSession, peekSessionMeta } from "@/lib/problem-session-store";
import { t } from "@/lib/i18n";
import { useLocale } from "@/lib/i18n-client";

interface Info {
  hasReal: boolean;
  lastUpdatedAt: number | null;
}

export function ResumeOrFresh({
  slug,
  align = "end",
}: {
  slug: string;
  align?: "start" | "center" | "end";
}) {
  useLocale();
  const router = useRouter();
  const [info, setInfo] = useState<Info | null>(null);
  const [handle, setHandle] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    Backend.me()
      .then((me) => {
        if (cancelled) return;
        setHandle(me.handle);
        const meta = peekSessionMeta(me.handle, slug);
        setInfo({
          hasReal: meta.hasRealWork,
          lastUpdatedAt: meta.lastUpdatedAt,
        });
      })
      .catch(() => {
        // Not signed in — let the workspace's own auth guard redirect.
        if (!cancelled) setInfo({ hasReal: false, lastUpdatedAt: null });
      });
    return () => {
      cancelled = true;
    };
  }, [slug]);

  const alignClass =
    align === "center" ? "items-center" : align === "start" ? "items-start" : "items-end";

  if (info === null) {
    return (
      <div className={`flex flex-col gap-2 ${alignClass}`}>
        <div className="h-10 w-44 rounded-md bg-surface-2 animate-pulse" />
      </div>
    );
  }

  if (!info.hasReal) {
    return (
      <div className={`flex flex-col gap-2 ${alignClass}`}>
        <Link href={`/problems/${slug}/workspace`}>
          <Button size="lg" className="gap-2">
            <Play size={15} strokeWidth={2} />
            {t("resume_start_solving_btn")}
          </Button>
        </Link>
      </div>
    );
  }

  const startFresh = () => {
    if (handle) deleteSession(handle, slug);
    // Pass intent through the URL so the workspace can call POST
    // /api/sessions with force_new=true on first mount.
    router.push(`/problems/${slug}/workspace?fresh=1`);
  };
  const resume = () => {
    router.push(`/problems/${slug}/workspace`);
  };

  return (
    <div className={`flex flex-col gap-2 ${alignClass}`}>
      <div className="flex gap-2 flex-wrap">
        <Button
          onClick={startFresh}
          variant="outline"
          size="lg"
          className="gap-2"
        >
          <RotateCcw size={14} strokeWidth={1.7} />
          {t("resume_start_fresh_btn")}
        </Button>
        <Button onClick={resume} size="lg" className="gap-2">
          <Play size={15} strokeWidth={2} />
          {t("resume_continue_btn")}
        </Button>
      </div>
      {info.lastUpdatedAt && (
        <div
          className="text-xs text-muted"
          title={formatDateTime(info.lastUpdatedAt)}
        >
          {t("resume_last_edited_fmt", { params: { when: formatRelativeTime(info.lastUpdatedAt) } })}
        </div>
      )}
    </div>
  );
}
