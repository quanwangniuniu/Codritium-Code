"use client";

import { useEffect, useState } from "react";

// Seconds since `startedAt` (ms epoch), ticking once a second. Stays 0
// until a start time is known.
export function useElapsedSeconds(startedAt: number | undefined): number {
  const [elapsed, setElapsed] = useState(0);
  useEffect(() => {
    if (startedAt === undefined) return;
    const id = setInterval(() => {
      setElapsed(Math.floor((Date.now() - startedAt) / 1000));
    }, 1000);
    return () => clearInterval(id);
  }, [startedAt]);
  return elapsed;
}
