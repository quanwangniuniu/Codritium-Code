// Reply schema mirrors the session_events shape so the same UI can replay
// either an authored official walkthrough (solutions_replies row) or the
// candidate's own session (session_events rows transformed to this shape).
//
// Source of truth for the 16 kinds is backend/internal/events/event.go.
// Surface kinds render as visible cards. Collapse kinds are stripped by
// the transform layer — they are useful for grading but noisy for replay.

export type ReplyKind =
  // surface — decision-rich, visible cards
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
  // collapse — transient / internal, hidden in the replay UI
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

// SURFACE_KINDS is the allowlist of kinds the replay UI renders. Anything
// outside this set is dropped during transform. Lift this to a constant so
// EnvelopeCard and any generator validators share the same vocabulary.
export const SURFACE_KINDS: ReadonlySet<ReplyKind> = new Set<ReplyKind>([
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

// transformForReplay drops collapse kinds and stable-sorts by seq so the
// envelopes feed StepController in the order the session actually emitted.
// Callers that already trust the source ordering can skip the sort, but the
// extra cost is negligible against a few dozen rows.
export function transformForReplay(envelopes: ReplyEnvelope[]): ReplyEnvelope[] {
  return envelopes
    .filter((env) => isSurfaceKind(env.kind))
    .slice()
    .sort((a, b) => a.seq - b.seq);
}
