"use client";

import { useEffect, useState } from "react";
import { subscribe, type ToastItem } from "@/shared/lib/toast";
import { Toast } from "./Toast";

export function ToastContainer() {
  const [items, setItems] = useState<ToastItem[]>([]);

  useEffect(() => {
    return subscribe(setItems);
  }, []);

  return (
    <>
      <style>{`
        @keyframes toast-slide-in {
          from { opacity: 0; transform: translateY(-8px); }
          to   { opacity: 1; transform: translateY(0); }
        }
      `}</style>
      <div
        aria-live="polite"
        style={{
          position: "fixed",
          top: 16,
          right: 16,
          display: "flex",
          flexDirection: "column",
          gap: 8,
          zIndex: 9999,
          pointerEvents: "none",
        }}
      >
        {items.map((item) => (
          <div key={item.id} style={{ pointerEvents: "auto" }}>
            <Toast item={item} />
          </div>
        ))}
      </div>
    </>
  );
}
