"use client";

import { useEffect, useRef, useState } from "react";

// Vertical splitter: a thin draggable bar that emits horizontal deltas.
// Apply VS Code-style hover/active highlight + global cursor lock so dragging
// across the editor or chat panel doesn't get hijacked by their own cursors.
export function Splitter({ onResize }: { onResize: (deltaX: number) => void }) {
  const lastX = useRef(0);
  const [dragging, setDragging] = useState(false);

  const handleMouseDown = (e: React.MouseEvent) => {
    setDragging(true);
    lastX.current = e.clientX;
    e.preventDefault();
  };

  useEffect(() => {
    if (!dragging) return;
    const onMove = (e: MouseEvent) => {
      const dx = e.clientX - lastX.current;
      if (dx !== 0) {
        lastX.current = e.clientX;
        onResize(dx);
      }
    };
    const onUp = () => setDragging(false);
    window.addEventListener("mousemove", onMove);
    window.addEventListener("mouseup", onUp);
    document.body.classList.add("is-resizing");
    return () => {
      window.removeEventListener("mousemove", onMove);
      window.removeEventListener("mouseup", onUp);
      document.body.classList.remove("is-resizing");
    };
  }, [dragging, onResize]);

  return (
    <div
      onMouseDown={handleMouseDown}
      className={`splitter-v${dragging ? " dragging" : ""}`}
      role="separator"
      aria-orientation="vertical"
    />
  );
}
