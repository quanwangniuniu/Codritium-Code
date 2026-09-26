"use client";

import { useEffect, useRef, useState, type ReactNode } from "react";
import { createPortal } from "react-dom";
import { X } from "lucide-react";

export interface FloatingPanelProps {
  title: string;
  children: ReactNode;
  onClose: () => void;
  initial?: { x: number; y: number; w: number; h: number };
}

const DEFAULT_INITIAL = { x: 140, y: 96, w: 540, h: 640 };

export function FloatingPanel({
  title,
  children,
  onClose,
  initial = DEFAULT_INITIAL,
}: FloatingPanelProps) {
  const [mounted, setMounted] = useState(false);
  const [pos, setPos] = useState(initial);
  const dragOffset = useRef<{ dx: number; dy: number } | null>(null);
  const resizeStart = useRef<{
    startX: number;
    startY: number;
    startW: number;
    startH: number;
  } | null>(null);

  useEffect(() => {
    setMounted(true);
  }, []);

  useEffect(() => {
    const onMove = (e: MouseEvent) => {
      if (dragOffset.current) {
        const d = dragOffset.current;
        setPos((p) => ({
          ...p,
          x: Math.max(0, e.clientX - d.dx),
          y: Math.max(0, e.clientY - d.dy),
        }));
      } else if (resizeStart.current) {
        const r = resizeStart.current;
        setPos((p) => ({
          ...p,
          w: Math.max(320, r.startW + (e.clientX - r.startX)),
          h: Math.max(240, r.startH + (e.clientY - r.startY)),
        }));
      }
    };
    const onUp = () => {
      dragOffset.current = null;
      resizeStart.current = null;
    };
    window.addEventListener("mousemove", onMove);
    window.addEventListener("mouseup", onUp);
    return () => {
      window.removeEventListener("mousemove", onMove);
      window.removeEventListener("mouseup", onUp);
    };
  }, []);

  const startDrag = (e: React.MouseEvent) => {
    if ((e.target as HTMLElement).closest("[data-floating-noclose]")) return;
    dragOffset.current = { dx: e.clientX - pos.x, dy: e.clientY - pos.y };
  };

  const startResize = (e: React.MouseEvent) => {
    e.stopPropagation();
    resizeStart.current = {
      startX: e.clientX,
      startY: e.clientY,
      startW: pos.w,
      startH: pos.h,
    };
  };

  if (!mounted || typeof document === "undefined") return null;

  return createPortal(
    <div
      style={{
        position: "fixed",
        left: pos.x,
        top: pos.y,
        width: pos.w,
        height: pos.h,
        background: "var(--bg-side)",
        border: "1px solid var(--border)",
        borderRadius: 8,
        boxShadow: "0 12px 32px rgba(0,0,0,0.45)",
        zIndex: 9999,
        display: "flex",
        flexDirection: "column",
        overflow: "hidden",
      }}
    >
      <div
        onMouseDown={startDrag}
        style={{
          cursor: "move",
          padding: "8px 12px",
          borderBottom: "1px solid var(--border)",
          background: "var(--bg-side)",
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          flexShrink: 0,
          userSelect: "none",
        }}
      >
        <span
          style={{
            fontSize: 12,
            color: "var(--text-strong)",
            fontWeight: 600,
          }}
        >
          {title}
        </span>
        <button
          data-floating-noclose
          onClick={onClose}
          title="Close"
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
          onMouseEnter={(e) => {
            e.currentTarget.style.background = "var(--bg-tab-hover)";
          }}
          onMouseLeave={(e) => {
            e.currentTarget.style.background = "transparent";
          }}
        >
          <X size={13} strokeWidth={1.7} />
        </button>
      </div>
      <div
        className="scroll-y"
        style={{
          flex: 1,
          overflow: "auto",
          background: "var(--bg-editor)",
        }}
      >
        {children}
      </div>
      <div
        onMouseDown={startResize}
        style={{
          position: "absolute",
          right: 0,
          bottom: 0,
          width: 18,
          height: 18,
          cursor: "nwse-resize",
          background:
            "linear-gradient(135deg, transparent 50%, var(--text-dim) 50%, var(--text-dim) 60%, transparent 60%, transparent 70%, var(--text-dim) 70%, var(--text-dim) 80%, transparent 80%)",
        }}
        title="Resize"
      />
    </div>,
    document.body,
  );
}
