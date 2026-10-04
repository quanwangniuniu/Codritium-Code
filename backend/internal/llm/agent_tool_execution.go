package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"codritium/backend/internal/actionworkflow"
	"codritium/backend/internal/events"

	"github.com/google/uuid"
)

// handleToolUse resolves and validates one action, records its proposal,
// obtains an automatic or candidate decision, and executes it when approved.
// Even on reject, a tool_result is appended so the model can react.
func (a *Agent) handleToolUse(ctx context.Context, sessionID uuid.UUID, turnIndex int, tu pendingToolUse) error {
	action, actionExists := defaultActionRegistry.Resolve(actionworkflow.Request{
		Name:  tu.Name,
		Input: actionworkflow.Input(tu.Input),
	})
	if !actionExists {
		return fmt.Errorf("unknown action: %s", tu.Name)
	}

	var validationContext actionworkflow.ValidationContext
	if a.Workspace != nil {
		validationContext = a.Workspace
	}

	validatedInput, validationErr := action.ValidateInput(
		actionworkflow.Input(tu.Input),
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
			actionworkflow.Input(modifiedInput),
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
