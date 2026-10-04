"use client";

import { useCallback, useMemo, useState } from "react";
import { workspaceApi } from "@/features/workspace/api";
import {
  applyToolResult,
  markPatchResolved,
  patchFromProposal,
  updateTextMessage,
} from "@/features/workspace/lib/workspace-state";
import type {
  ChatMessage,
  PendingPatch,
  StreamEnvelope,
} from "@/features/workspace/types";
import { t } from "@/shared/i18n";
import { toast } from "@/shared/lib/toast";
import type { SetSessionFn } from "./useProblemSession";
import { useSessionStream } from "./useSessionStream";

interface SessionEventsOptions {
  sessionId: string | null;
  fileContents: Record<string, string>;
  setSession: SetSessionFn;
  onProposal: (patch: PendingPatch) => void;
  onDecided: (toolUseId: string) => void;
}

// Agent chat over the session SSE stream: sends a turn, streams assistant
// text into the placeholder message, and folds tool proposals / decisions
// / turn completion envelopes into the chat timeline.
export function useSessionEvents({
  sessionId,
  fileContents,
  setSession,
  onProposal,
  onDecided,
}: SessionEventsOptions) {
  const [busy, setBusy] = useState(false);
  const [streamingAssistantId, setStreamingAssistantId] = useState<
    string | null
  >(null);

  const onEnvelope = useCallback(
    (env: StreamEnvelope) => {
      switch (env.kind) {
        case "tool_use_proposed": {
          const patch = patchFromProposal(env, fileContents);
          const auto = patch.auto === true;
          if (!auto) onProposal(patch);
          setSession((prev) => ({
            ...prev,
            messages: [
              ...prev.messages,
              {
                kind: "patch",
                id: `p-${patch.toolUseId}`,
                pending: patch,
                ...(auto ? { resolved: { kind: "approve" as const } } : {}),
              },
            ],
          }));
          break;
        }
        case "tool_result": {
          const toolUseId = String(env.payload.tool_use_id ?? "");
          const durationValue = env.payload.duration_ms;
          const durationMs =
            typeof durationValue === "number" ? durationValue : undefined;

          const result = {
            summary: String(env.payload.output_summary ?? ""),
            isError: env.payload.is_error === true,
            durationMs,
          };

          setSession((prev) => applyToolResult(prev, toolUseId, result));
          break;
        }
        case "candidate_approved":
        case "candidate_rejected": {
          const id = String(env.payload.tool_use_id ?? "");
          const decisionKind =
            env.kind === "candidate_rejected"
              ? "reject"
              : env.payload.modified === true
                ? "modify"
                : "approve";

          onDecided(id);
          setSession((prev) => ({
            ...prev,
            messages: markPatchResolved(prev.messages, id, decisionKind),
          }));
          break;
        }
        case "turn_completed": {
          if (streamingAssistantId) {
            setSession((prev) => ({
              ...prev,
              messages: updateTextMessage(
                prev.messages,
                streamingAssistantId,
                (m) => ({
                  ...m,
                  streaming: false,
                }),
              ),
            }));
            setStreamingAssistantId(null);
          }
          setBusy(false);
          break;
        }
      }
    },
    [fileContents, streamingAssistantId, setSession, onProposal, onDecided],
  );

  const onTextDelta = useCallback(
    (delta: string) => {
      if (!streamingAssistantId) return;
      setSession((prev) => ({
        ...prev,
        messages: updateTextMessage(
          prev.messages,
          streamingAssistantId,
          (m) => ({
            ...m,
            content: m.content + delta,
          }),
        ),
      }));
    },
    [streamingAssistantId, setSession],
  );

  const streamOpts = useMemo(
    () => ({ onEnvelope, onTextDelta }),
    [onEnvelope, onTextDelta],
  );
  useSessionStream(sessionId, streamOpts);

  const send = async (text: string) => {
    if (!sessionId) {
      toast.warn("Session not ready yet. Try again in a moment.");
      return;
    }
    const userMsg: ChatMessage = {
      id: `u-${Date.now()}`,
      role: "user",
      content: text,
    };
    const assistantId = `a-${Date.now()}`;
    const assistantMsg: ChatMessage = {
      id: assistantId,
      role: "assistant",
      content: "",
      streaming: true,
    };
    setSession((prev) => ({
      ...prev,
      messages: [...prev.messages, userMsg, assistantMsg],
    }));
    setStreamingAssistantId(assistantId);
    setBusy(true);
    try {
      await workspaceApi.chat(sessionId, text, fileContents);
    } catch (e) {
      console.error("chat send failed:", e);
      setSession((prev) => ({
        ...prev,
        messages: prev.messages.filter((m) => m.id !== assistantId),
      }));
      setStreamingAssistantId(null);
      setBusy(false);
      toast.error(t("chat_send_failed"));
    }
  };

  return { busy, send };
}
