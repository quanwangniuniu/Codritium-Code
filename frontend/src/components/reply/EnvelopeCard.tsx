"use client";

import {
  Wrench,
  CheckCircle2,
  XCircle,
  CornerDownLeft,
  PlayCircle,
  FileSearch,
  Pencil,
  Terminal,
  Clipboard,
  Flag,
  Undo2,
} from "lucide-react";
import type { ReplyEnvelope, ReplyKind } from "@/types/reply";
import { SURFACE_KINDS } from "@/types/reply";
import { t } from "@/shared/i18n";
import { useLocale } from "@/shared/i18n/client";

// EnvelopeCard renders a single replay step. It is the only place the UI
// knows what each of the 13 surface kinds looks like; the StepController
// and the higher-level page only see "render(env)". Collapse kinds
// (compact_triggered / first_message_classified / ai_output_read) return
// null, which the transform layer would normally filter out before we
// get here — the guard is belt-and-suspenders.

type Side = "ai" | "candidate" | "center" | "full";

interface CardLayout {
  side: Side;
  accent: string;
  background: string;
  border: string;
}

const AI_ACCENT = "#3b82f6";        // calm blue for AI / tool side
const CANDIDATE_ACCENT = "#22c55e"; // green for candidate decisions
const NEUTRAL_ACCENT = "#9ca3af";   // grey for state transitions
const WARNING_ACCENT = "#f59e0b";   // amber for reverts and pushbacks
const DANGER_ACCENT = "#ef4444";    // red for rejects

const AI_SIDE: CardLayout = {
  side: "ai",
  accent: AI_ACCENT,
  background: "rgba(59, 130, 246, 0.08)",
  border: "rgba(59, 130, 246, 0.3)",
};

const CANDIDATE_SIDE: CardLayout = {
  side: "candidate",
  accent: CANDIDATE_ACCENT,
  background: "rgba(34, 197, 94, 0.08)",
  border: "rgba(34, 197, 94, 0.3)",
};

function pickKindPayload(env: ReplyEnvelope): {
  description: string;
  tool_name?: string;
  summary?: string;
  decision_reason?: string;
  code_snippet?: string;
} {
  const p = env.payload as Record<string, unknown>;
  return {
    description: typeof p.description === "string" ? p.description : "",
    tool_name: typeof p.tool_name === "string" ? p.tool_name : undefined,
    summary: typeof p.summary === "string" ? p.summary : undefined,
    decision_reason: typeof p.decision_reason === "string" ? p.decision_reason : undefined,
    code_snippet: typeof p.code_snippet === "string" ? p.code_snippet : undefined,
  };
}

export function EnvelopeCard({ env }: { env: ReplyEnvelope }) {
  useLocale();
  if (!SURFACE_KINDS.has(env.kind)) return null;
  const payload = pickKindPayload(env);
  return <KindCard env={env} payload={payload} />;
}

function KindCard({
  env,
  payload,
}: {
  env: ReplyEnvelope;
  payload: ReturnType<typeof pickKindPayload>;
}) {
  switch (env.kind) {
    case "session_started":
      return (
        <CenteredChip
          icon={<Flag size={12} />}
          label={t("envelope_session_started")}
          accent={NEUTRAL_ACCENT}
          subtitle={payload.description}
        />
      );
    case "session_submitted":
      return (
        <CenteredChip
          icon={<Flag size={12} />}
          label={t("envelope_session_submitted")}
          accent={NEUTRAL_ACCENT}
          subtitle={payload.description}
        />
      );
    case "plan_mode_entered":
      return (
        <CenteredChip
          icon={<Clipboard size={12} />}
          label={t("envelope_plan_in")}
          accent={NEUTRAL_ACCENT}
          subtitle={payload.description}
        />
      );
    case "plan_mode_exited":
      return (
        <CenteredChip
          icon={<Clipboard size={12} />}
          label={t("envelope_plan_out")}
          accent={NEUTRAL_ACCENT}
          subtitle={payload.description}
        />
      );
    case "tool_use_proposed":
      return (
        <SideBubble
          layout={AI_SIDE}
          who={t("ai_role")}
          icon={iconForTool(payload.tool_name)}
          title={
            payload.tool_name
              ? t("envelope_propose_with_tool", { params: { tool: payload.tool_name } })
              : t("envelope_propose_tool")
          }
          summary={payload.summary}
          description={payload.description}
        />
      );
    case "tool_result":
      return (
        <SideBubble
          layout={AI_SIDE}
          who={t("ai_role")}
          icon={<Wrench size={13} strokeWidth={2} />}
          title={t("envelope_tool_result")}
          summary={payload.summary}
          description={payload.description}
          muted
        />
      );
    case "turn_completed":
      return (
        <SideBubble
          layout={CANDIDATE_SIDE}
          who={t("role_engineer")}
          icon={<CornerDownLeft size={13} strokeWidth={2} />}
          title={t("envelope_reasoning")}
          description={payload.description}
        />
      );
    case "candidate_approved":
      return (
        <DecisionPill
          icon={<CheckCircle2 size={13} strokeWidth={2} />}
          label={t("envelope_approved")}
          accent={CANDIDATE_ACCENT}
          reason={payload.decision_reason}
          description={payload.description}
        />
      );
    case "candidate_rejected":
      return (
        <DecisionPill
          icon={<XCircle size={13} strokeWidth={2} />}
          label={t("envelope_rejected")}
          accent={DANGER_ACCENT}
          reason={payload.decision_reason}
          description={payload.description}
        />
      );
    case "candidate_pushed_back":
      return (
        <DecisionPill
          icon={<CornerDownLeft size={13} strokeWidth={2} />}
          label={t("envelope_pushed_back")}
          accent={WARNING_ACCENT}
          reason={payload.decision_reason}
          description={payload.description}
        />
      );
    case "test_executed":
      return (
        <FullCard
          icon={<PlayCircle size={14} strokeWidth={2} color={AI_ACCENT} />}
          title={t("envelope_tests_executed")}
          accent={AI_ACCENT}
          summary={payload.summary}
          description={payload.description}
          snippet={payload.code_snippet}
        />
      );
    case "self_check_artifact":
      return (
        <FullCard
          icon={<FileSearch size={14} strokeWidth={2} color={CANDIDATE_ACCENT} />}
          title={t("envelope_self_check")}
          accent={CANDIDATE_ACCENT}
          summary={payload.summary}
          description={payload.description}
          snippet={payload.code_snippet}
        />
      );
    case "candidate_reverted_edit":
      return (
        <DecisionPill
          icon={<Undo2 size={13} strokeWidth={2} />}
          label={t("envelope_reverted")}
          accent={WARNING_ACCENT}
          reason={payload.decision_reason}
          description={payload.description}
        />
      );
    default:
      return null;
  }
}

function iconForTool(name: string | undefined): React.ReactNode {
  switch (name) {
    case "FileRead":
      return <FileSearch size={13} strokeWidth={2} />;
    case "FileEdit":
      return <Pencil size={13} strokeWidth={2} />;
    case "RunTests":
      return <Terminal size={13} strokeWidth={2} />;
    default:
      return <Wrench size={13} strokeWidth={2} />;
  }
}

function CenteredChip({
  icon,
  label,
  accent,
  subtitle,
}: {
  icon: React.ReactNode;
  label: string;
  accent: string;
  subtitle?: string;
}) {
  return (
    <div style={{ display: "flex", flexDirection: "column", alignItems: "center", gap: 4 }}>
      <span
        style={{
          display: "inline-flex",
          alignItems: "center",
          gap: 5,
          padding: "3px 10px",
          borderRadius: 999,
          background: "var(--surface-2)",
          color: accent,
          fontSize: 11,
          fontWeight: 600,
          letterSpacing: 0.3,
          textTransform: "uppercase",
        }}
      >
        {icon}
        {label}
      </span>
      {subtitle && (
        <div
          style={{
            fontSize: 11.5,
            color: "var(--muted)",
            textAlign: "center",
            maxWidth: 440,
            lineHeight: 1.5,
          }}
        >
          {subtitle}
        </div>
      )}
    </div>
  );
}

function SideBubble({
  layout,
  who,
  icon,
  title,
  summary,
  description,
  muted,
}: {
  layout: CardLayout;
  who: string;
  icon: React.ReactNode;
  title: string;
  summary?: string;
  description: string;
  muted?: boolean;
}) {
  const align = layout.side === "candidate" ? "flex-end" : "flex-start";
  return (
    <div style={{ display: "flex", flexDirection: "column", alignItems: align, gap: 4 }}>
      <div
        style={{
          fontSize: 11,
          color: "var(--muted)",
          paddingLeft: 4,
          paddingRight: 4,
        }}
      >
        {who}
      </div>
      <div
        style={{
          maxWidth: "82%",
          background: muted ? "var(--surface-2)" : layout.background,
          border: `1px solid ${layout.border}`,
          borderRadius: 8,
          padding: "9px 12px",
          display: "flex",
          flexDirection: "column",
          gap: 6,
        }}
      >
        <div
          style={{
            display: "flex",
            alignItems: "center",
            gap: 6,
            fontSize: 12.5,
            fontWeight: 600,
            color: layout.accent,
          }}
        >
          {icon}
          {title}
        </div>
        {summary && (
          <div
            style={{
              fontSize: 12,
              fontFamily: "var(--font-jetbrains-mono), ui-monospace, monospace",
              color: "var(--ink)",
              background: "var(--surface)",
              padding: "4px 8px",
              borderRadius: 4,
              border: "1px solid var(--divider)",
            }}
          >
            {summary}
          </div>
        )}
        {description && (
          <div style={{ fontSize: 12.5, color: "var(--ink)", lineHeight: 1.55 }}>
            {description}
          </div>
        )}
      </div>
    </div>
  );
}

function DecisionPill({
  icon,
  label,
  accent,
  reason,
  description,
}: {
  icon: React.ReactNode;
  label: string;
  accent: string;
  reason?: string;
  description: string;
}) {
  return (
    <div style={{ display: "flex", flexDirection: "column", alignItems: "flex-end", gap: 4 }}>
      <span
        style={{
          display: "inline-flex",
          alignItems: "center",
          gap: 5,
          padding: "3px 10px",
          borderRadius: 999,
          background: `${accent}22`,
          color: accent,
          fontSize: 11,
          fontWeight: 600,
          letterSpacing: 0.3,
          textTransform: "uppercase",
        }}
      >
        {icon}
        {label}
      </span>
      {(reason || description) && (
        <div
          style={{
            maxWidth: "82%",
            fontSize: 12.5,
            color: "var(--ink)",
            background: "var(--surface)",
            border: `1px solid ${accent}44`,
            borderRadius: 8,
            padding: "8px 12px",
            lineHeight: 1.55,
            display: "flex",
            flexDirection: "column",
            gap: 4,
          }}
        >
          {reason && (
            <div style={{ fontSize: 11.5, color: "var(--muted)" }}>{reason}</div>
          )}
          {description && <div>{description}</div>}
        </div>
      )}
    </div>
  );
}

function FullCard({
  icon,
  title,
  accent,
  summary,
  description,
  snippet,
}: {
  icon: React.ReactNode;
  title: string;
  accent: string;
  summary?: string;
  description: string;
  snippet?: string;
}) {
  return (
    <div
      style={{
        background: "var(--surface)",
        border: `1px solid ${accent}44`,
        borderRadius: 8,
        padding: "10px 14px",
        display: "flex",
        flexDirection: "column",
        gap: 8,
      }}
    >
      <div
        style={{
          display: "flex",
          alignItems: "center",
          gap: 6,
          fontSize: 12.5,
          fontWeight: 600,
          color: accent,
        }}
      >
        {icon}
        {title}
      </div>
      {summary && (
        <div
          style={{
            fontSize: 12,
            fontFamily: "var(--font-jetbrains-mono), ui-monospace, monospace",
            color: "var(--ink)",
            background: "var(--surface-2)",
            padding: "4px 8px",
            borderRadius: 4,
          }}
        >
          {summary}
        </div>
      )}
      {description && (
        <div style={{ fontSize: 12.5, color: "var(--ink)", lineHeight: 1.55 }}>
          {description}
        </div>
      )}
      {snippet && (
        <pre
          style={{
            background: "var(--surface-2)",
            border: "1px solid var(--divider)",
            borderRadius: 4,
            padding: "8px 10px",
            margin: 0,
            fontSize: 11.5,
            fontFamily: "var(--font-jetbrains-mono), ui-monospace, monospace",
            overflowX: "auto",
            whiteSpace: "pre-wrap",
            wordBreak: "break-word",
          }}
        >
          {snippet}
        </pre>
      )}
    </div>
  );
}

// Re-export to expose the ReplyKind for any caller that needs to dispatch
// further (e.g. the toggle button at the top of the page may render kind
// counts for the timeline summary).
export type { ReplyKind };
