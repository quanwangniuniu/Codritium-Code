// agent_loop.go — candidate-agent main turn loop.
//
// Spec: PLAN/v0.8/design/chat_loop_skeleton.md
//
// One Agent runs one candidate session. RunTurn executes one user-message
// → assistant-response cycle, possibly looping internally through several
// tool_use roundtrips. Every state transition goes through events.Store —
// callers do not consume a channel from this package; they subscribe to
// the store. (decision_log.md D6: this keeps event ordering vs transcript
// ordering totally ordered via the shared per-session seq counter.)
//
// The loop has three exits per turn: normal (model stops without further
// tool_use), max_turns (iteration count exceeds Agent.MaxIterations), and
// aborted (context cancel / stream error / decision timeout). Each exit
// emits exactly one turn_completed event with the matching reason.
package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"codritium/backend/internal/agentworkflow"
	"codritium/backend/internal/events"
)

// Agent owns the per-session state needed to run RunTurn. One Agent is
// long-lived for the session; RunTurn is called once per user message.
type Agent struct {
	Stream        LLMStreamClient
	Events        *events.Store
	Text          *TextBroadcaster // optional — chat text streamed here (spec §5 keeps it off the events table)
	Waiter        *DecisionWaiter
	Tools         Registry
	Workspace     *Workspace
	Sandbox       SandboxRunner
	TmpFS         *SessionTmpFS // per-session scratch dir for Grep / Glob / RunCommand
	SystemPrompt  string
	Deny          []DenyRule
	VisibleTest   string
	MaxIterations int // safety guard; default 50 if zero

	// running history (carried across turns within the session)
	messages []Message
	// turnMu serialises RunTurn: each chat message starts its own goroutine,
	// and two turns must never interleave on messages or the workspace.
	turnMu sync.Mutex

	closeMu sync.Mutex
	closed  chan struct{} // closed by Close; lazily created
}

func (a *Agent) closedCh() chan struct{} {
	a.closeMu.Lock()
	defer a.closeMu.Unlock()
	if a.closed == nil {
		a.closed = make(chan struct{})
	}
	return a.closed
}

// Close cancels any in-flight RunTurn (including one blocked waiting on a
// candidate decision). Safe to call more than once.
func (a *Agent) Close() {
	ch := a.closedCh()
	a.closeMu.Lock()
	defer a.closeMu.Unlock()
	select {
	case <-ch:
	default:
		close(ch)
	}
}

// RunResult is what RunTurn returns to the caller after the turn ends.
type RunResult struct {
	Reason     string // "normal" | "max_turns" | "aborted"
	Iterations int
	Usage      events.TokenUsage
}

const (
	defaultMaxIterations     = 20
	maxRepeatedAutoToolCalls = 3
)

var defaultActionRegistry = agentworkflow.NewDefaultRegistry()

// RunTurn consumes one user message and runs the agent until either the
// model stops asking for tools (normal exit) or a guard / failure trips
// (max_turns / aborted). All events are written to the store; the caller
// observes the session via events.Store.Subscribe.
func (a *Agent) RunTurn(ctx context.Context, sessionID uuid.UUID, turnIndex int, userMsg string) (RunResult, error) {
	a.turnMu.Lock()
	defer a.turnMu.Unlock()

	maxIter := a.MaxIterations
	if maxIter <= 0 {
		maxIter = defaultMaxIterations
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		select {
		case <-a.closedCh():
			cancel()
		case <-ctx.Done():
		}
	}()

	a.recordMessage(ctx, sessionID, Message{
		Role:    "user",
		Content: []ContentBlock{{Kind: "text", Text: userMsg}},
	})

	usage := events.TokenUsage{}
	iter := 0

	lastAutoToolKey := ""
	repeatedAutoToolCalls := 0

	for {
		if err := ctx.Err(); err != nil {
			return a.completeAborted(ctx, sessionID, turnIndex, iter, usage, "ctx: "+err.Error())
		}
		iter++
		if iter > maxIter {
			return a.completeMaxTurns(ctx, sessionID, turnIndex, iter, usage)
		}

		// 1) call the model
		req := TurnRequest{
			SystemPrompt: a.SystemPrompt,
			Messages:     a.messages,
			Tools:        a.toolSpecs(),
		}
		stream, err := a.Stream.StreamTurn(ctx, req)
		if err != nil {
			return a.completeAborted(ctx, sessionID, turnIndex, iter, usage, "stream: "+err.Error())
		}

		// 2) drain the stream into one assistant message + collected tool_uses
		assistantMsg, pending, turnUsage, stopReason, drainErr := a.drainStream(ctx, sessionID, stream)
		_ = stream.Close()
		if drainErr != nil {
			return a.completeAborted(ctx, sessionID, turnIndex, iter, usage, "drain: "+drainErr.Error())
		}
		usage = addUsage(usage, turnUsage)
		a.recordMessage(ctx, sessionID, assistantMsg)

		// 3) no tool_use → turn ends normally
		if len(pending) == 0 {
			_ = stopReason // currently informational only
			return a.completeNormal(ctx, sessionID, turnIndex, iter, usage)
		}

		// 4) each tool_use: validate → propose → approve or wait → execute
		for _, tu := range pending {
			action, actionExists := defaultActionRegistry.Resolve(agentworkflow.Request{
				Name:  tu.Name,
				Input: agentworkflow.Input(tu.Input),
			})
			if actionExists && !action.RequiresApproval {
				key := tu.Name + ":" + hashFor(tu.Input)
				if key == lastAutoToolKey {
					repeatedAutoToolCalls++
				} else {
					lastAutoToolKey = key
					repeatedAutoToolCalls = 1
				}

				if repeatedAutoToolCalls > maxRepeatedAutoToolCalls {
					output, _ := json.Marshal(map[string]any{
						"error": "repeated identical read blocked",
						"detail": fmt.Sprintf(
							"%s was called repeatedly with the same input; use the existing result, choose a different tool, or finish the task",
							tu.Name,
						),
					})

					a.recordMessage(ctx, sessionID, Message{
						Role: "tool",
						Content: []ContentBlock{{
							Kind: "tool_result",
							ToolResult: &ContentToolResult{
								ToolUseID: tu.ID,
								Output:    string(output),
								IsError:   true,
							},
						}},
					})

					return a.completeMaxTurns(
						ctx,
						sessionID,
						turnIndex,
						iter,
						usage,
					)
				}
			} else {
				lastAutoToolKey = ""
				repeatedAutoToolCalls = 0
			}

			if err := a.handleToolUse(ctx, sessionID, turnIndex, tu); err != nil {
				return a.completeAborted(
					ctx,
					sessionID,
					turnIndex,
					iter,
					usage,
					"tool_use: "+err.Error(),
				)
			}
		}
		// loop back: model will see new tool_result messages on next iteration
	}
}

// recordMessage appends msg to the running history and persists it to the
// session transcript (session_messages), which the grader reads at submit
// time. A persistence failure is logged, not fatal: the turn keeps going
// with the in-memory history intact.
func (a *Agent) recordMessage(ctx context.Context, sessionID uuid.UUID, msg Message) {
	a.messages = append(a.messages, msg)
	if a.Events == nil {
		return
	}
	content, err := json.Marshal(anthropicBlocks(msg.Content))
	if err != nil {
		log.Printf("agent loop: marshal transcript message for %s: %v", sessionID, err)
		return
	}
	if err := a.Events.AppendMessage(ctx, sessionID, msg.Role, content); err != nil {
		log.Printf("agent loop: persist transcript message for %s: %v", sessionID, err)
	}
}

// anthropicBlocks renders content blocks in the Anthropic wire shape
// session_messages.content is documented to hold.
func anthropicBlocks(blocks []ContentBlock) []map[string]any {
	out := make([]map[string]any, 0, len(blocks))
	for _, b := range blocks {
		switch {
		case b.Kind == "text":
			out = append(out, map[string]any{"type": "text", "text": b.Text})
		case b.Kind == "tool_use" && b.ToolUse != nil:
			out = append(out, map[string]any{
				"type":  "tool_use",
				"id":    b.ToolUse.ID,
				"name":  b.ToolUse.Name,
				"input": b.ToolUse.Input,
			})
		case b.Kind == "tool_result" && b.ToolResult != nil:
			out = append(out, map[string]any{
				"type":        "tool_result",
				"tool_use_id": b.ToolResult.ToolUseID,
				"content":     b.ToolResult.Output,
				"is_error":    b.ToolResult.IsError,
			})
		}
	}
	return out
}

// PendingToolUse is what drainStream collects from one streamed completion.
type pendingToolUse struct {
	ID    string
	Name  string
	Input map[string]any
}

func (a *Agent) drainStream(ctx context.Context, sessionID uuid.UUID, r StreamReader) (Message, []pendingToolUse, events.TokenUsage, string, error) {
	msg := Message{Role: "assistant"}
	var (
		usage      events.TokenUsage
		stopReason string
		pendingMap = map[string]*pendingBuilder{} // tool_use_id → accumulator
		order      []string                       // ordered tool_use_id list
		textBuf    strings.Builder
	)
	for {
		chunk, ok, err := r.Next(ctx)
		if err != nil {
			return msg, nil, usage, "", err
		}
		if !ok {
			break
		}
		switch chunk.Kind {
		case "text_delta":
			textBuf.WriteString(chunk.Text)
			if a.Text != nil {
				a.Text.Publish(sessionID, chunk.Text)
			}
		case "tool_use_start":
			pb := &pendingBuilder{name: chunk.ToolUseName}
			pendingMap[chunk.ToolUseID] = pb
			order = append(order, chunk.ToolUseID)
		case "tool_use_input_delta":
			if pb := pendingMap[chunk.ToolUseID]; pb != nil {
				pb.input.WriteString(chunk.InputJSONDelta)
			}
		case "tool_use_stop":
			// no-op: input fully assembled at this point
		case "usage":
			usage = events.TokenUsage{
				InputTokens:         chunk.InputTokens,
				OutputTokens:        chunk.OutputTokens,
				CacheCreationTokens: chunk.CacheCreationTokens,
				CacheReadTokens:     chunk.CacheReadTokens,
			}
		case "message_stop":
			stopReason = chunk.StopReason
		}
	}

	// 1) text block (if any)
	if textBuf.Len() > 0 {
		msg.Content = append(msg.Content, ContentBlock{Kind: "text", Text: textBuf.String()})
	}
	// 2) tool_use blocks (in arrival order)
	pending := make([]pendingToolUse, 0, len(order))
	for _, id := range order {
		pb := pendingMap[id]
		var input map[string]any
		raw := pb.input.String()
		if raw != "" {
			if err := json.Unmarshal([]byte(raw), &input); err != nil {
				return msg, nil, usage, stopReason, fmt.Errorf("decode tool input %s: %w", id, err)
			}
		}
		if input == nil {
			input = map[string]any{}
		}
		msg.Content = append(msg.Content, ContentBlock{
			Kind:    "tool_use",
			ToolUse: &ContentToolUse{ID: id, Name: pb.name, Input: input},
		})
		pending = append(pending, pendingToolUse{ID: id, Name: pb.name, Input: input})
	}
	return msg, pending, usage, stopReason, nil
}

type pendingBuilder struct {
	name  string
	input strings.Builder
}

// handleToolUse resolves and validates one action, records its proposal,
// obtains an automatic or candidate decision, and executes it when approved.
// Even on reject, a tool_result is appended so the model can react.
func (a *Agent) handleToolUse(ctx context.Context, sessionID uuid.UUID, turnIndex int, tu pendingToolUse) error {
	action, actionExists := defaultActionRegistry.Resolve(agentworkflow.Request{
		Name:  tu.Name,
		Input: agentworkflow.Input(tu.Input),
	})
	if !actionExists {
		return fmt.Errorf("unknown action: %s", tu.Name)
	}

	var validationContext agentworkflow.ValidationContext
	if a.Workspace != nil {
		validationContext = a.Workspace
	}

	validatedInput, validationErr := action.ValidateInput(
		agentworkflow.Input(tu.Input),
		validationContext,
	)
	if validationErr != nil {
		a.recordInvalidToolUse(
			ctx,
			sessionID,
			tu.ID,
			tu.Name,
			validationErr.Error(),
		)
		return nil
	}
	tu.Input = map[string]any(validatedInput)

	summary := summarizeInput(tu.Name, tu.Input)
	inputHash := hashFor(tu.Input)
	isAuto := !action.RequiresApproval

	if _, err := a.Events.Append(ctx, sessionID, events.ToolUseProposed{
		ToolUseID:    tu.ID,
		Tool:         tu.Name,
		Input:        tu.Input,
		InputSummary: summary,
		InputHash:    inputHash,
		TurnIndex:    turnIndex,
		Auto:         isAuto,
	}); err != nil {
		return err
	}

	var decision Decision
	if isAuto {
		decision = Decision{Kind: "approve"}
	} else {
		d, err := a.Waiter.Wait(ctx, DecisionKey(sessionID, tu.ID))
		if err != nil {
			return fmt.Errorf("waiter: %w", err)
		}
		decision = d
	}

	// emit the decision event
	switch decision.Kind {
	case "approve":
		_, _ = a.Events.Append(ctx, sessionID, events.CandidateApproved{
			ToolUseID:               tu.ID,
			Modified:                decision.ModifiedInput != "",
			ModificationExcerpt:     excerpt(decision.ModifiedInput),
			CandidateCommentExcerpt: excerpt(decision.Comment),
			Auto:                    isAuto,
		})
	case "modify":
		_, _ = a.Events.Append(ctx, sessionID, events.CandidateApproved{
			ToolUseID:               tu.ID,
			Modified:                true,
			ModificationExcerpt:     excerpt(decision.ModifiedInput),
			CandidateCommentExcerpt: excerpt(decision.Comment),
		})
	case "reject":
		kind := "no_reason"
		if decision.Reason != "" {
			kind = "with_reason"
		}
		_, _ = a.Events.Append(ctx, sessionID, events.CandidateRejected{
			ToolUseID:     tu.ID,
			ReasonExcerpt: excerpt(decision.Reason),
			ReasonKind:    kind,
		})
	}

	if decision.Kind == "reject" {
		a.recordMessage(ctx, sessionID, Message{
			Role: "tool",
			Content: []ContentBlock{{
				Kind: "tool_result",
				ToolResult: &ContentToolResult{
					ToolUseID: tu.ID,
					Output:    fmt.Sprintf(`{"rejected_by_candidate":true,"reason":%q}`, decision.Reason),
					IsError:   true,
				},
			}},
		})
		return nil
	}

	// approve / modify path: pick and verify the effective input, then execute
	effectiveInput := tu.Input
	if decision.Kind == "modify" && decision.ModifiedInput != "" {
		var modifiedInput map[string]any
		if err := json.Unmarshal(
			[]byte(decision.ModifiedInput),
			&modifiedInput,
		); err != nil {
			a.recordInvalidToolUse(
				ctx,
				sessionID,
				tu.ID,
				tu.Name,
				"modified input is not valid JSON",
			)
			return nil
		}

		validatedModifiedInput, err := action.ValidateInput(
			agentworkflow.Input(modifiedInput),
			validationContext,
		)
		if err != nil {
			a.recordInvalidToolUse(
				ctx,
				sessionID,
				tu.ID,
				tu.Name,
				err.Error(),
			)
			return nil
		}

		effectiveInput = map[string]any(validatedModifiedInput)
	}

	tool, ok := a.Tools[tu.Name]
	if !ok {
		return fmt.Errorf("unknown tool: %s", tu.Name)
	}
	t0 := time.Now()
	output, isErr, execErr := tool.Execute(ctx, effectiveInput, ToolDeps{
		Workspace:       a.Workspace,
		Sandbox:         a.Sandbox,
		Deny:            a.Deny,
		VisibleTestFile: a.VisibleTest,
		TmpFS:           a.TmpFS,
	})
	if execErr != nil {
		return execErr
	}
	if err := action.ValidateResult(output, isErr); err != nil {
		return fmt.Errorf("verify %s result: %w", tu.Name, err)
	}
	dur := time.Since(t0).Milliseconds()

	_, _ = a.Events.Append(ctx, sessionID, events.ToolResult{
		ToolUseID:     tu.ID,
		IsError:       isErr,
		OutputSummary: excerpt(output),
		OutputHash:    hashFor(output),
		DurationMs:    dur,
	})

	a.recordMessage(ctx, sessionID, Message{
		Role: "tool",
		Content: []ContentBlock{{
			Kind: "tool_result",
			ToolResult: &ContentToolResult{
				ToolUseID: tu.ID,
				Output:    output,
				IsError:   isErr,
			},
		}},
	})
	return nil
}

func (a *Agent) recordInvalidToolUse(
	ctx context.Context,
	sessionID uuid.UUID,
	toolUseID string,
	toolName string,
	detail string,
) {
	output, _ := json.Marshal(map[string]string{
		"error":  "invalid " + toolName,
		"detail": detail,
	})

	a.recordMessage(ctx, sessionID, Message{
		Role: "tool",
		Content: []ContentBlock{{
			Kind: "tool_result",
			ToolResult: &ContentToolResult{
				ToolUseID: toolUseID,
				Output:    string(output),
				IsError:   true,
			},
		}},
	})
}

func (a *Agent) toolSpecs() []ToolSpec {
	out := make([]ToolSpec, 0, len(a.Tools))
	for _, t := range a.Tools {
		out = append(out, t.Spec())
	}
	return out
}

func (a *Agent) completeNormal(ctx context.Context, sid uuid.UUID, turn, iter int, usage events.TokenUsage) (RunResult, error) {
	_, _ = a.Events.Append(ctx, sid, events.TurnCompleted{
		TurnIndex: turn, Iterations: iter, Usage: usage, Reason: "normal",
	})
	return RunResult{Reason: "normal", Iterations: iter, Usage: usage}, nil
}

func (a *Agent) completeMaxTurns(ctx context.Context, sid uuid.UUID, turn, iter int, usage events.TokenUsage) (RunResult, error) {
	_, _ = a.Events.Append(ctx, sid, events.TurnCompleted{
		TurnIndex: turn, Iterations: iter, Usage: usage, Reason: "max_turns",
	})
	return RunResult{Reason: "max_turns", Iterations: iter, Usage: usage}, nil
}

func (a *Agent) completeAborted(ctx context.Context, sid uuid.UUID, turn, iter int, usage events.TokenUsage, why string) (RunResult, error) {
	_, _ = a.Events.Append(ctx, sid, events.TurnCompleted{
		TurnIndex: turn, Iterations: iter, Usage: usage, Reason: "aborted:" + why,
	})
	return RunResult{Reason: "aborted", Iterations: iter, Usage: usage}, nil
}

func addUsage(a, b events.TokenUsage) events.TokenUsage {
	return events.TokenUsage{
		InputTokens:         a.InputTokens + b.InputTokens,
		OutputTokens:        a.OutputTokens + b.OutputTokens,
		CacheCreationTokens: a.CacheCreationTokens + b.CacheCreationTokens,
		CacheReadTokens:     a.CacheReadTokens + b.CacheReadTokens,
	}
}

// ─── helpers ────────────────────────────────────────────────────────────

const excerptCap = 200

func excerpt(s string) string {
	if len(s) <= excerptCap {
		return s
	}
	return s[:excerptCap]
}

func hashFor(v any) string {
	// Cheap deterministic hash: serialise then SHA-like — kept tiny on
	// purpose. The real grader joins on session_messages for full text,
	// so the hash only needs to be a stable fingerprint for the row.
	b, _ := json.Marshal(v)
	// FNV would do; std lib hash/fnv would import for one line. Use a
	// simple djb2 variant — collisions are tolerable because session_id
	// + seq is the actual PK.
	var h uint64 = 5381
	for _, c := range b {
		h = ((h << 5) + h) + uint64(c)
	}
	return fmt.Sprintf("djb2:%x", h)
}

func summarizeInput(toolName string, input map[string]any) string {
	switch toolName {
	case "FileEdit":
		path, _ := input["path"].(string)
		content, _ := input["content"].(string)
		if newText, ok := input["new_text"].(string); ok {
			content = newText
		}
		return fmt.Sprintf("Edit %s (%d bytes)", path, len(content))
	case "FileRead":
		path, _ := input["path"].(string)
		return fmt.Sprintf("Read %s", path)
	case "RunTests":
		path, _ := input["path"].(string)
		if path == "" {
			return "Run tests"
		}
		return fmt.Sprintf("Run %s", path)
	case "Grep":
		pattern, _ := input["pattern"].(string)
		glob, _ := input["path_glob"].(string)
		if glob != "" {
			return fmt.Sprintf("Grep %q in %s", pattern, glob)
		}
		return fmt.Sprintf("Grep %q", pattern)
	case "Glob":
		pattern, _ := input["pattern"].(string)
		return fmt.Sprintf("Glob %s", pattern)
	case "RunCommand":
		cmd, _ := input["command"].(string)
		if len(cmd) > 80 {
			cmd = cmd[:80] + "…"
		}
		return fmt.Sprintf("Run: %s", cmd)
	}
	return toolName
}
