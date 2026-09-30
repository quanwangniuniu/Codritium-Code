"use client";

import { useEffect, useState } from "react";
import { usePathname, useRouter } from "next/navigation";
import { createPortal } from "react-dom";
import { ArrowLeft, X } from "lucide-react";
import { t } from "@/shared/i18n";
import { useLocale } from "@/shared/i18n/client";

// Top-left workspace control. Leaves the IDE and returns to the problem brief
// (/problems/[id]); it does NOT sign the user out. In-progress edits persist
// through the session store, so leaving is non-destructive.
export function ExitWorkspaceButton() {
  useLocale();
  const router = useRouter();
  const pathname = usePathname();
  const [open, setOpen] = useState(false);

  // The workspace lives at /problems/[id]/workspace; the brief is its parent.
  const introHref = pathname.replace(/\/workspace\/?$/, "") || "/problems";

  const handleLeave = () => {
    setOpen(false);
    router.push(introHref);
  };

  return (
    <>
      <button
        onClick={() => setOpen(true)}
        title={t("workspace_exit_aria")}
        aria-label={t("workspace_exit_aria")}
        style={{
          display: "inline-flex",
          alignItems: "center",
          justifyContent: "center",
          width: 28,
          height: 28,
          color: "var(--text)",
          background: "transparent",
          border: "none",
          borderRadius: 3,
          cursor: "pointer",
        }}
        onMouseEnter={(e) => {
          e.currentTarget.style.background = "var(--bg-tab-hover)";
        }}
        onMouseLeave={(e) => {
          e.currentTarget.style.background = "transparent";
        }}
      >
        <ArrowLeft size={15} strokeWidth={1.8} />
      </button>
      {open && (
        <ExitModal onCancel={() => setOpen(false)} onConfirm={handleLeave} />
      )}
    </>
  );
}

function ExitModal({
  onCancel,
  onConfirm,
}: {
  onCancel: () => void;
  onConfirm: () => void;
}) {
  const [mounted, setMounted] = useState(false);

  useEffect(() => {
    setMounted(true);
  }, []);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onCancel();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onCancel]);

  if (!mounted || typeof document === "undefined") return null;

  return createPortal(
    <div
      onClick={(e) => {
        if (e.target === e.currentTarget) onCancel();
      }}
      style={{
        position: "fixed",
        inset: 0,
        background: "rgba(0,0,0,0.55)",
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        zIndex: 10000,
      }}
    >
      <div
        role="dialog"
        aria-modal="true"
        aria-labelledby="exit-modal-title"
        style={{
          width: "min(440px, 90vw)",
          background: "var(--bg-side)",
          border: "1px solid var(--border)",
          borderRadius: 8,
          boxShadow: "0 16px 40px rgba(0,0,0,0.5)",
          overflow: "hidden",
        }}
      >
        <div
          style={{
            display: "flex",
            alignItems: "center",
            justifyContent: "space-between",
            padding: "12px 16px",
            borderBottom: "1px solid var(--border)",
          }}
        >
          <h2
            id="exit-modal-title"
            style={{
              fontSize: 14,
              fontWeight: 600,
              color: "var(--text-strong)",
              margin: 0,
            }}
          >
            {t("workspace_exit_title")}
          </h2>
          <button
            onClick={onCancel}
            aria-label={t("cancel")}
            style={{
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              width: 22,
              height: 22,
              color: "var(--text-dim)",
              background: "transparent",
              border: "none",
              borderRadius: 3,
              cursor: "pointer",
            }}
          >
            <X size={13} strokeWidth={1.7} />
          </button>
        </div>
        <div
          style={{
            padding: "16px 18px",
            fontSize: 13,
            color: "var(--text)",
            lineHeight: 1.6,
          }}
        >
          {t("workspace_exit_body")}
        </div>
        <div
          style={{
            display: "flex",
            justifyContent: "flex-end",
            gap: 8,
            padding: "12px 16px",
            borderTop: "1px solid var(--border)",
            background: "var(--bg-side-header)",
          }}
        >
          <button
            onClick={onCancel}
            style={{
              fontSize: 12,
              padding: "6px 14px",
              color: "var(--text)",
              background: "transparent",
              border: "1px solid var(--border-strong)",
              borderRadius: 4,
              cursor: "pointer",
            }}
          >
            {t("cancel")}
          </button>
          <button
            onClick={onConfirm}
            style={{
              fontSize: 12,
              fontWeight: 600,
              padding: "6px 14px",
              color: "var(--accent-fg)",
              background: "var(--accent)",
              border: "none",
              borderRadius: 4,
              cursor: "pointer",
            }}
          >
            <span className="inline-flex items-center gap-[5px]">
              <ArrowLeft size={12} strokeWidth={1.9} />
              {t("workspace_exit_action")}
            </span>
          </button>
        </div>
      </div>
    </div>,
    document.body,
  );
}
