// ReplyEnvelope is the shared replay shape used by official walkthroughs
// and candidate sessions. Candidate replays combine transcript messages
// from session_messages with structured events from session_events.

export type ReplyKind =
  // surface — visible replay cards
  | "chat_message"
  | "session_started"
  | "turn_completed"
  | "session_submitted"
  | "tool_use_proposed"
  | "tool_result"
  | "candidate_approved"
  | "candidate_rejected"
  | "candidate_pushed_back"
  | "plan_mode_entered"
  | "plan_mode_exited"
  | "test_executed"
  | "self_check_artifact"
  | "candidate_reverted_edit"
  // collapse — internal events hidden from replay
  | "compact_triggered"
  | "first_message_classified"
  | "ai_output_read";

export interface ReplyEnvelope {
  session_id: string;
  seq: number;
  kind: ReplyKind;
  emitted_at: string;
  payload: Record<string, unknown>;
}

export type ReplySource = "official_ai" | "user_share" | "user_session";

export interface ReplyMeta {
  source: ReplySource;
  challenge_slug: string;
  generated_at?: string;
  generator_model?: string;
  candidate_index?: number;
}

export interface Reply {
  meta: ReplyMeta;
  envelopes: ReplyEnvelope[];
}

// SURFACE_KINDS contains the replay steps displayed to the candidate.
export const SURFACE_KINDS: ReadonlySet<ReplyKind> = new Set<ReplyKind>([
  "chat_message",
  "session_started",
  "turn_completed",
  "session_submitted",
  "tool_use_proposed",
  "tool_result",
  "candidate_approved",
  "candidate_rejected",
  "candidate_pushed_back",
  "plan_mode_entered",
  "plan_mode_exited",
  "test_executed",
  "self_check_artifact",
  "candidate_reverted_edit",
]);

export function isSurfaceKind(kind: ReplyKind): boolean {
  return SURFACE_KINDS.has(kind);
}

// transformForReplay removes internal events and restores the original
// ordering shared by transcript messages and structured events.
export function transformForReplay(
  envelopes: ReplyEnvelope[],
): ReplyEnvelope[] {
  return envelopes
    .filter((env) => isSurfaceKind(env.kind))
    .slice()
    .sort((a, b) => a.seq - b.seq);
}
