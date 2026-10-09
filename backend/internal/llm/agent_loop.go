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
	"strings"
	"sync"

	"github.com/google/uuid"

	"codritium/backend/internal/actionworkflow"
	"codritium/backend/internal/events"
)

// Agent owns the per-session state needed to run RunTurn. One Agent is
// long-lived for the session; RunTurn is called once per user message.
type Agent struct {
	Stream          LLMStreamClient
	Events          *events.Store
	Text            *TextBroadcaster // optional — chat text streamed here (spec §5 keeps it off the events table)
	Waiter          *DecisionWaiter
	Tools           Registry
	Workspace       *Workspace
	Sandbox         SandboxRunner
	TmpFS           *SessionTmpFS // per-session scratch dir for Grep / Glob / RunCommand
	SystemPrompt    string
	Deny            []DenyRule
	VisibleTest     string
	MaxIterations   int // safety guard; default 20 if zero
	MaxSearchCalls  int // per-turn Glob/Grep guard; default 4 if zero
	MaxFileReads    int // per-turn FileRead guard; default 12 if zero
	MaxOutputTokens int // per-model-call output limit; default 1024 if zero
	Context         *ContextManager

	// running history (carried across turns within the session)
	messages []Message
	// turnMu serialises RunTurn: each chat message starts its own goroutine,
	// and two turns must never interleave on messages or the workspace.
	turnMu sync.Mutex

	closeMu sync.Mutex
	closed  chan struct{} // closed by Close; lazily created
}

// contextManager returns the session context manager, creating it lazily
// for agents constructed directly in tests.
func (a *Agent) contextManager() *ContextManager {
	if a.Context == nil {
		a.Context = NewContextManager()
	}
	return a.Context
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
	defaultMaxSearchCalls    = 4
	defaultMaxFileReads      = 12
	defaultMaxOutputTokens   = 1024
	maxRepeatedAutoToolCalls = 3
	maxEmptyResponseRetries  = 3
)

var defaultActionRegistry = actionworkflow.NewDefaultRegistry()

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

	maxSearchCalls := a.MaxSearchCalls
	if maxSearchCalls <= 0 {
		maxSearchCalls = defaultMaxSearchCalls
	}

	maxFileReads := a.MaxFileReads
	if maxFileReads <= 0 {
		maxFileReads = defaultMaxFileReads
	}

	maxOutputTokens := a.MaxOutputTokens
	if maxOutputTokens <= 0 {
		maxOutputTokens = defaultMaxOutputTokens
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
	emptyResponseRetries := 0

	searchCalls := 0
	searchBudgetExhausted := false

	fileReadCalls := 0
	fileReadBudgetExhausted := false

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

		// 1) prepare the bounded model context, then call the model.
		preparedMessages, compactStats := a.contextManager().Prepare(
			a.messages,
		)
		if compactStats != nil {
			a.messages = preparedMessages

			if a.Events != nil {
				_, _ = a.Events.Append(
					ctx,
					sessionID,
					events.CompactTriggered{
						Reason:          "auto",
						RemovedMessages: compactStats.RemovedMessages,
						TokensBefore:    compactStats.TokensBefore,
						TokensAfter:     compactStats.TokensAfter,
					},
				)
			}
		}

		systemPrompt := a.SystemPrompt
		latestResult := latestToolResult(a.messages)

		if emptyResponseRetries > 0 {
			systemPrompt += "\n\nThe previous response was empty. Continue from the latest " +
				"tool result and either call the next required tool or explain the result."
		}

		if strings.Contains(latestResult, "test file") &&
			strings.Contains(latestResult, "not found in workspace") {
			systemPrompt += "\n\nThe requested test file does not exist. Create it first " +
				"with FileEdit, wait for approval, and then call RunTests again with " +
				"the same path. Do not stop after explaining this."
		}

		if strings.Contains(latestResult, `"already_available":true`) {
			systemPrompt += "\n\nThe latest tool result is already available in the context. " +
				"Do not repeat the same tool call. Use the existing result and continue " +
				"with the investigation, required FileEdit operations, and RunTests. " +
				"Do not stop after receiving an already-available result."
		}

		if strings.Contains(latestResult, `"rejected_by_candidate":true`) {
			systemPrompt += "\n\nThe previous tool call was rejected and was not applied. " +
				"Do not claim that the rejected change was completed. Review the rejection " +
				"reason, choose a different approach, and call the appropriate FileEdit or " +
				"RunTests tool. Continue until the requested work is complete."
		}

		if searchBudgetExhausted {
			systemPrompt += "\n\nThe search-tool budget is exhausted. Do not call Glob or Grep again " +
				"in this turn. Use the visible workspace file list and existing search results, " +
				"then read only the relevant files and continue with the fix and tests."
		}

		if fileReadBudgetExhausted {
			systemPrompt += "\n\nThe FileRead budget is exhausted. Do not call FileRead again in this turn. " +
				"Use the code and file ranges already available in the context. Proceed with the necessary " +
				"FileEdit operations and RunTests instead of reading more files."
		}

		req := TurnRequest{
			SystemPrompt:    systemPrompt,
			Messages:        preparedMessages,
			Tools:           a.toolSpecs(),
			MaxOutputTokens: maxOutputTokens,
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

		if len(assistantMsg.Content) == 0 && len(pending) == 0 {
			if emptyResponseRetries < maxEmptyResponseRetries {
				emptyResponseRetries++
				continue
			}

			if a.Text != nil {
				a.Text.Publish(
					sessionID,
					"The local model repeatedly returned an empty response. Please retry.",
				)
			}

			return a.completeAborted(
				ctx,
				sessionID,
				turnIndex,
				iter,
				usage,
				"empty model response",
			)
		}

		emptyResponseRetries = 0
		a.recordMessage(ctx, sessionID, assistantMsg)

		// 3) no tool_use → turn ends normally
		if len(pending) == 0 {
			_ = stopReason // currently informational only
			return a.completeNormal(ctx, sessionID, turnIndex, iter, usage)
		}

		// 4) each tool_use: validate → propose → approve or wait → execute
		for _, tu := range pending {
			action, actionExists := defaultActionRegistry.Resolve(actionworkflow.Request{
				Name:  tu.Name,
				Input: actionworkflow.Input(tu.Input),
			})
			if isSearchTool(tu.Name) {
				if searchCalls >= maxSearchCalls {
					searchBudgetExhausted = true
					a.recordSearchBudgetExceeded(
						ctx,
						sessionID,
						tu.ID,
					)
					continue
				}
				searchCalls++
				if searchCalls >= maxSearchCalls {
					searchBudgetExhausted = true
				}
			}
			if tu.Name == "FileRead" {
				if fileReadCalls >= maxFileReads {
					fileReadBudgetExhausted = true
					a.recordFileReadBudgetExceeded(
						ctx,
						sessionID,
						tu.ID,
					)
					continue
				}
				fileReadCalls++
				if fileReadCalls >= maxFileReads {
					fileReadBudgetExhausted = true
				}
			}
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

					if tu.Name == "FileRead" {
						fileReadBudgetExhausted = true
					}
					if isSearchTool(tu.Name) {
						searchBudgetExhausted = true
					}

					repeatedAutoToolCalls = 0
					continue
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
		startLine, hasStart, _ := optionalPositiveInt(
			input,
			"start_line",
		)
		lineCount, hasCount, _ := optionalPositiveInt(
			input,
			"line_count",
		)

		if hasStart && hasCount {
			return fmt.Sprintf(
				"Read %s lines %d-%d",
				path,
				startLine,
				startLine+lineCount-1,
			)
		}
		if hasStart {
			return fmt.Sprintf(
				"Read %s from line %d",
				path,
				startLine,
			)
		}
		if hasCount {
			return fmt.Sprintf(
				"Read first %d lines of %s",
				lineCount,
				path,
			)
		}
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
