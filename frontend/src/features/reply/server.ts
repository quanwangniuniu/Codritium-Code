// Server-side replay reads: the official solution run, the viewer's latest
// run on a problem, and the run behind a specific submission.
import { apiJSON } from "@/shared/api/server";

// Reply payload shape mirrors what /api/challenges/{slug}/official-reply
// returns. envelopes is kept as the raw ReplyEnvelope[] so the client
// component can run transformForReplay on it. files / starter_files /
// explanation_md feed the two-column replay UI: file tree, cumulative
// diff baseline, and standard-solution prose respectively.
export interface OfficialReplyResponse {
  source: string;
  challenge_slug: string;
  generator_model?: string;
  candidate_index?: number;
  created_at?: string;
  envelopes: unknown[];
  files?: Record<string, string>;
  starter_files?: Record<string, string>;
  explanation_md?: string;
}

export async function getOfficialReply(
  slug: string,
): Promise<OfficialReplyResponse | null> {
  return apiJSON<OfficialReplyResponse>(
    `/api/challenges/${encodeURIComponent(slug)}/official-reply`,
  );
}

// MyReplayResponse reuses the same envelope shape as the official reply
// but is fed from session_events for the candidate's most recent run on
// the slug. Returns null when the candidate has not started the problem
// yet (backend responds 404, the apiJSON helper translates to null).
export interface MyReplayResponse extends OfficialReplyResponse {
  session_id?: string;
  started_at?: string;
}

export async function getMyReplay(
  slug: string,
): Promise<MyReplayResponse | null> {
  return apiJSON<MyReplayResponse>(
    `/api/me/replays/${encodeURIComponent(slug)}`,
  );
}

// Per-session replay for a specific submission's run, keyed by the
// submission's session_id (so a candidate with several attempts sees the
// run that belongs to the submission they opened, not just the latest).
// Returns null when the session has no recorded events.
export async function getSessionReply(
  sessionId: string,
): Promise<MyReplayResponse | null> {
  return apiJSON<MyReplayResponse>(
    `/api/sessions/${encodeURIComponent(sessionId)}/reply`,
  );
}
