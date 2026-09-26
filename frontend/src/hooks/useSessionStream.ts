"use client";

import { useEffect, useRef, useState } from "react";
import { Backend } from "@/lib/api";

export interface StreamEnvelope {
  session_id: string;
  seq: number;
  kind: string;
  emitted_at: string;
  payload: Record<string, unknown>;
}

export interface UseSessionStreamOpts {
  onEnvelope?: (env: StreamEnvelope) => void;
  onTextDelta?: (delta: string) => void;
}

// useSessionStream opens a single EventSource to
// GET /api/sessions/{id}/stream and demultiplexes the two SSE event
// types — "agent_event" (structured envelopes) and "message" (free
// assistant text). Both reach the caller through optional callbacks so
// downstream state stays caller-owned. Auto-reconnect with backoff on
// error; idempotent close on unmount; idle when sessionId is null.
export function useSessionStream(
  sessionId: string | null,
  opts: UseSessionStreamOpts = {},
): { connected: boolean } {
  const [connected, setConnected] = useState(false);
  const esRef = useRef<EventSource | null>(null);
  const optsRef = useRef(opts);
  // Keep latest callbacks live without resubscribing on every render.
  useEffect(() => {
    optsRef.current = opts;
  }, [opts]);

  useEffect(() => {
    if (!sessionId) return;
    let cancelled = false;
    let retry: ReturnType<typeof setTimeout> | null = null;

    const connect = () => {
      if (cancelled) return;
      const es = new EventSource(Backend.streamURL(sessionId), { withCredentials: true });
      esRef.current = es;

      es.onopen = () => setConnected(true);
      es.onerror = () => {
        setConnected(false);
        es.close();
        if (!cancelled) retry = setTimeout(connect, 1500);
      };
      es.addEventListener("agent_event", (e) => {
        try {
          const env = JSON.parse((e as MessageEvent).data) as StreamEnvelope;
          optsRef.current.onEnvelope?.(env);
        } catch {
          /* ignore malformed */
        }
      });
      es.addEventListener("message", (e) => {
        try {
          const { text } = JSON.parse((e as MessageEvent).data) as { text: string };
          if (text) optsRef.current.onTextDelta?.(text);
        } catch {
          /* ignore */
        }
      });
    };

    connect();
    return () => {
      cancelled = true;
      if (retry) clearTimeout(retry);
      if (esRef.current) {
        esRef.current.close();
        esRef.current = null;
      }
      setConnected(false);
    };
  }, [sessionId]);

  return { connected };
}
