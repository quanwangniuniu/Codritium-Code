// Package events models the 14 structured event kinds the v0.8
// candidate-agent emits and persists for the grader to consume.
//
// Spec: PLAN/v0.8/design/events_payload_spec.md
//
// Each event type satisfies Event by providing a stable Kind() string (one of
// the 14 well-known kinds the grader knows about) and a Payload() that is
// JSON-marshalled into session_events.payload (JSONB) without any framing
// container. Adding a new kind requires updating both the design spec and the
// grader contracts; do not invent kinds at the call site.
package events

import "github.com/google/uuid"

// Event is the contract every emitted event satisfies.
type Event interface {
	// Kind returns one of the 14 well-known kind strings.
	Kind() string
	// Payload returns the JSON-marshallable body that lands in
	// session_events.payload. By convention it is the event struct itself.
	Payload() any
}

// Envelope is what subscribers and persistence both see — the event after
// the store has assigned its sequence number.
type Envelope struct {
	SessionID uuid.UUID `json:"session_id"`
	Seq       int64     `json:"seq"`
	Kind      string    `json:"kind"`
	EmittedAt string    `json:"emitted_at"` // RFC3339Nano
	Payload   any       `json:"payload"`
}

// ─── 1. Session lifecycle (4 kinds) ─────────────────────────────────────

type SessionStarted struct {
	ChallengeID       string   `json:"challenge_id"`
	Difficulty        string   `json:"difficulty"`
	CandidateRole     string   `json:"candidate_role,omitempty"`
	CandidateRoleText string   `json:"candidate_role_text,omitempty"`
	StarterFiles      []string `json:"starter_files,omitempty"`
	VisibleTestFile   string   `json:"visible_test_file,omitempty"`
	SystemPromptHash  string   `json:"system_prompt_hash,omitempty"`
}

func (SessionStarted) Kind() string  { return "session_started" }
func (e SessionStarted) Payload() any { return e }

type CompactTriggered struct {
	Reason          string `json:"reason"` // "auto" | "reactive"
	RemovedMessages int    `json:"removed_messages"`
	TokensBefore    int    `json:"tokens_before"`
	TokensAfter     int    `json:"tokens_after"`
}

func (CompactTriggered) Kind() string  { return "compact_triggered" }
func (e CompactTriggered) Payload() any { return e }

type TurnCompleted struct {
	TurnIndex  int        `json:"turn_index"`
	Iterations int        `json:"iterations"`
	Usage      TokenUsage `json:"usage"`
	Reason     string     `json:"reason"` // "normal" | "aborted" | "max_turns" | "budget"
}

func (TurnCompleted) Kind() string  { return "turn_completed" }
func (e TurnCompleted) Payload() any { return e }

type TokenUsage struct {
	InputTokens         int `json:"input_tokens"`
	OutputTokens        int `json:"output_tokens"`
	CacheCreationTokens int `json:"cache_creation_tokens,omitempty"`
	CacheReadTokens     int `json:"cache_read_tokens,omitempty"`
}

type SessionSubmitted struct {
	FinalFilesHash  string   `json:"final_files_hash"`
	TotalTurns      int      `json:"total_turns"`
	TotalDurationMs int64    `json:"total_duration_ms"`
	FilesChanged    []string `json:"files_changed,omitempty"`
}

func (SessionSubmitted) Kind() string  { return "session_submitted" }
func (e SessionSubmitted) Payload() any { return e }

// ─── 2. Main dialog loop (3 kinds) ──────────────────────────────────────

type ToolUseProposed struct {
	ToolUseID     string `json:"tool_use_id"`
	Tool          string `json:"tool"` // FileRead | FileEdit | RunTests | ...
	InputSummary  string `json:"input_summary"`
	InputHash     string `json:"input_hash,omitempty"`
	RefMessageID  string `json:"ref_message_id,omitempty"`
	TurnIndex     int    `json:"turn_index"`
	// Auto signals that the runtime decided to execute this tool without
	// waiting for a candidate decision (read-class tools like FileRead).
	// AI Collaboration scoring skips auto rows so the signal stays clean.
	Auto          bool   `json:"auto,omitempty"`
}

func (ToolUseProposed) Kind() string  { return "tool_use_proposed" }
func (e ToolUseProposed) Payload() any { return e }

type ToolResult struct {
	ToolUseID     string `json:"tool_use_id"`
	IsError       bool   `json:"is_error"`
	OutputSummary string `json:"output_summary"`
	OutputHash    string `json:"output_hash,omitempty"`
	DurationMs    int64  `json:"duration_ms,omitempty"`
}

func (ToolResult) Kind() string  { return "tool_result" }
func (e ToolResult) Payload() any { return e }

type FirstMessageClassified struct {
	Classification    string `json:"classification"` // bare_request | partial_plan | structured_plan
	ReasoningExcerpt  string `json:"reasoning_excerpt,omitempty"`
	ClassifierModel   string `json:"classifier_model,omitempty"`
}

func (FirstMessageClassified) Kind() string  { return "first_message_classified" }
func (e FirstMessageClassified) Payload() any { return e }

// ─── 3. Candidate decisions (3 kinds, AI Collaboration core) ────────────

type CandidateApproved struct {
	ToolUseID               string `json:"tool_use_id"`
	Modified                bool   `json:"modified"`
	ModificationExcerpt     string `json:"modification_excerpt,omitempty"`
	CandidateCommentExcerpt string `json:"candidate_comment_excerpt,omitempty"`
	// Auto = true means the runtime auto-approved this tool use (read-class
	// tools). Grader treats auto rows as system noise, not candidate signal.
	Auto                    bool   `json:"auto,omitempty"`
}

func (CandidateApproved) Kind() string  { return "candidate_approved" }
func (e CandidateApproved) Payload() any { return e }

type CandidateRejected struct {
	ToolUseID     string `json:"tool_use_id"`
	ReasonExcerpt string `json:"reason_excerpt,omitempty"`
	ReasonKind    string `json:"reason_kind"` // with_reason | no_reason
}

func (CandidateRejected) Kind() string  { return "candidate_rejected" }
func (e CandidateRejected) Payload() any { return e }

type CandidatePushedBack struct {
	OriginalToolUseID string `json:"original_tool_use_id"`
	PushbackExcerpt   string `json:"pushback_excerpt"`
	SecondProposalID  string `json:"second_proposal_id,omitempty"`
}

func (CandidatePushedBack) Kind() string  { return "candidate_pushed_back" }
func (e CandidatePushedBack) Payload() any { return e }

// ─── 4. Plan-mode transitions (2 kinds) ─────────────────────────────────

type PlanModeEntered struct {
	Trigger   string `json:"trigger"` // candidate_request | agent_suggestion
	TurnIndex int    `json:"turn_index"`
}

func (PlanModeEntered) Kind() string  { return "plan_mode_entered" }
func (e PlanModeEntered) Payload() any { return e }

type PlanModeExited struct {
	PlanStepsCount int    `json:"plan_steps_count"`
	PlanTextHash   string `json:"plan_text_hash,omitempty"`
	DurationMs     int64  `json:"duration_ms,omitempty"`
}

func (PlanModeExited) Kind() string  { return "plan_mode_exited" }
func (e PlanModeExited) Payload() any { return e }

// ─── 5. Verification (2 kinds, Verification core) ───────────────────────

type TestExecuted struct {
	Trigger          string   `json:"trigger"` // candidate | agent
	Command          string   `json:"command"`
	Passed           int      `json:"passed"`
	Failed           int      `json:"failed"`
	VisibleTestNames []string `json:"visible_test_names,omitempty"`
	DurationMs       int64    `json:"duration_ms,omitempty"`
}

func (TestExecuted) Kind() string  { return "test_executed" }
func (e TestExecuted) Payload() any { return e }

type SelfCheckArtifact struct {
	ArtifactType string `json:"artifact_type"` // invariant_comment | manual_scenario | self_test | contract_check
	Location     string `json:"location"`
	Excerpt      string `json:"excerpt,omitempty"`
}

func (SelfCheckArtifact) Kind() string  { return "self_check_artifact" }
func (e SelfCheckArtifact) Payload() any { return e }

// ─── 6. Candidate reading / reverting (R5 Gonfire absorption) ────────────

// AIOutputRead is emitted by the frontend after a candidate dwelt on an
// assistant message for a measurable amount of time. Designed to
// distinguish "blind accept" from "actually read it" for the AI
// Collaboration dimension. pause_duration is truncated to whole seconds
// — sub-second precision is intentionally discarded for privacy.
type AIOutputRead struct {
	MessageID          string `json:"message_id"`
	PauseDurationSec   int    `json:"pause_duration_sec"`
	ScrollDepthPercent int    `json:"scroll_depth_percent"`
	NextActionKind     string `json:"next_action_kind"` // new_prompt | tool_use_review | manual_edit | idle_timeout
}

func (AIOutputRead) Kind() string  { return "ai_output_read" }
func (e AIOutputRead) Payload() any { return e }

// CandidateRevertedEdit is emitted when the candidate undoes a previously
// applied edit. Distinct from candidate_rejected (which fires before the
// edit lands) — see events_payload_spec.md §5.5.2 for the edge cases.
type CandidateRevertedEdit struct {
	OriginalToolUseID string   `json:"original_tool_use_id"`
	RevertMethod      string   `json:"revert_method"` // undo | git_revert | overwrite | manual_undo
	FilesAffected     []string `json:"files_affected,omitempty"`
	NextActionKind    string   `json:"next_action_kind,omitempty"` // new_prompt | manual_edit | reject_new_proposal | exit_plan_mode
	NextActionRef     string   `json:"next_action_ref,omitempty"`
}

func (CandidateRevertedEdit) Kind() string  { return "candidate_reverted_edit" }
func (e CandidateRevertedEdit) Payload() any { return e }

// KnownKinds is the closed set of valid event kinds (handy for tests).
var KnownKinds = map[string]struct{}{
	"session_started":          {},
	"compact_triggered":        {},
	"turn_completed":           {},
	"session_submitted":        {},
	"tool_use_proposed":        {},
	"tool_result":              {},
	"first_message_classified": {},
	"candidate_approved":       {},
	"candidate_rejected":       {},
	"candidate_pushed_back":    {},
	"plan_mode_entered":        {},
	"plan_mode_exited":         {},
	"test_executed":            {},
	"self_check_artifact":      {},
	"ai_output_read":           {}, // R5: candidate dwell on assistant message
	"candidate_reverted_edit":  {}, // R5: candidate undoes applied edit
}
