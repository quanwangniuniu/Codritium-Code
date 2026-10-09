package llm

import (
	"context"
	"encoding/json"
	"log"

	"github.com/google/uuid"
)

func latestToolResult(messages []Message) string {
	for messageIndex := len(messages) - 1; messageIndex >= 0; messageIndex-- {
		blocks := messages[messageIndex].Content

		for blockIndex := len(blocks) - 1; blockIndex >= 0; blockIndex-- {
			result := blocks[blockIndex].ToolResult
			if blocks[blockIndex].Kind == "tool_result" &&
				result != nil {
				return result.Output
			}
		}
	}

	return ""
}

func isSearchTool(name string) bool {
	return name == "Glob" || name == "Grep"
}

func (a *Agent) recordSearchBudgetExceeded(
	ctx context.Context,
	sessionID uuid.UUID,
	toolUseID string,
) {
	output, _ := json.Marshal(map[string]any{
		"search_budget_exhausted": true,
		"detail": "The Glob/Grep budget for this turn is exhausted. " +
			"Use the existing results and continue with relevant FileRead, FileEdit, and test tools.",
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

func (a *Agent) recordFileReadBudgetExceeded(
	ctx context.Context,
	sessionID uuid.UUID,
	toolUseID string,
) {
	output, _ := json.Marshal(map[string]any{
		"file_read_budget_exhausted": true,
		"detail": "The FileRead budget for this turn is exhausted. " +
			"Use the existing file contents and continue with FileEdit and RunTests.",
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
