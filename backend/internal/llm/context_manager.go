package llm

import (
	"encoding/json"
	"fmt"
	"strings"
)

const (
	defaultContextBudgetTokens = 8000
	defaultContextKeepMessages = 8
	contextCharsPerToken       = 4
	maxContextSummaryChars     = 6000
	maxRetainedBlockChars      = 4000
)

// ContextManager controls the message history sent to the model.
// The complete transcript remains stored separately in session_messages.
type ContextManager struct {
	BudgetTokens       int
	KeepRecentMessages int
	readRanges         map[string]string
	searchQueries      map[string]string
}

type CompactionStats struct {
	RemovedMessages int
	TokensBefore    int
	TokensAfter     int
}

func NewContextManager() *ContextManager {
	return &ContextManager{
		BudgetTokens:       defaultContextBudgetTokens,
		KeepRecentMessages: defaultContextKeepMessages,
		readRanges:         make(map[string]string),
		searchQueries:      make(map[string]string),
	}
}

// AlreadyRead reports whether the same range of the same file version has
// already been returned to the model.
func (m *ContextManager) AlreadyRead(
	input map[string]any,
	workspace *Workspace,
) bool {
	key, version, ok := readRangeIdentity(input, workspace)
	if !ok {
		return false
	}
	return m.readRanges[key] == version
}

// RememberRead records a successful FileRead result.
func (m *ContextManager) RememberRead(
	input map[string]any,
	workspace *Workspace,
) {
	key, version, ok := readRangeIdentity(input, workspace)
	if !ok {
		return
	}
	if m.readRanges == nil {
		m.readRanges = make(map[string]string)
		m.searchQueries = make(map[string]string)
	}
	m.readRanges[key] = version
}

// AlreadySearched reports whether the same search has already run against
// the current workspace version.
func (m *ContextManager) AlreadySearched(
	toolName string,
	input map[string]any,
	workspace *Workspace,
) bool {
	key, version, ok := searchIdentity(toolName, input, workspace)
	if !ok {
		return false
	}
	return m.searchQueries[key] == version
}

// RememberSearch records a successful Glob or Grep result.
func (m *ContextManager) RememberSearch(
	toolName string,
	input map[string]any,
	workspace *Workspace,
) {
	key, version, ok := searchIdentity(toolName, input, workspace)
	if !ok {
		return
	}
	if m.searchQueries == nil {
		m.searchQueries = make(map[string]string)
	}
	m.searchQueries[key] = version
}

// Prepare returns the history that should be sent to the model. It does not
// modify the persisted session transcript.
func (m *ContextManager) Prepare(
	messages []Message,
) ([]Message, *CompactionStats) {
	budget := m.BudgetTokens
	if budget <= 0 {
		budget = defaultContextBudgetTokens
	}

	tokensBefore := estimateMessageTokens(messages)
	if tokensBefore <= budget {
		return messages, nil
	}

	keep := m.KeepRecentMessages
	if keep <= 0 {
		keep = defaultContextKeepMessages
	}
	if keep >= len(messages) {
		keep = len(messages) - 1
	}
	if keep < 1 {
		keep = 1
	}

	cut := len(messages) - keep

	// Never begin retained history with an unmatched tool result.
	for cut < len(messages) && messages[cut].Role == "tool" {
		cut++
	}

	summary := summarizeMessages(messages[:cut])
	retained := compactRetainedMessages(
		messages[cut:],
		maxRetainedBlockChars,
	)

	compacted := make([]Message, 0, len(retained)+1)
	compacted = append(compacted, Message{
		Role: "user",
		Content: []ContentBlock{{
			Kind: "text",
			Text: "[Compacted context summary]\n" + summary,
		}},
	})
	compacted = append(compacted, retained...)

	compacted = enforceContextBudget(compacted, budget)

	// Old FileRead results may have been removed from the model context.
	// Allow the model to request those ranges again when details are needed.
	m.readRanges = make(map[string]string)

	return compacted, &CompactionStats{
		RemovedMessages: cut,
		TokensBefore:    tokensBefore,
		TokensAfter:     estimateMessageTokens(compacted),
	}
}

func searchIdentity(
	toolName string,
	input map[string]any,
	workspace *Workspace,
) (string, string, bool) {
	if toolName != "Glob" && toolName != "Grep" {
		return "", "", false
	}
	if workspace == nil {
		return "", "", false
	}

	key := toolName + ":" + hashFor(input)
	version := hashFor(workspace.Files)
	return key, version, true
}

func readRangeIdentity(
	input map[string]any,
	workspace *Workspace,
) (string, string, bool) {
	if workspace == nil {
		return "", "", false
	}

	path, _ := input["path"].(string)
	if path == "" {
		return "", "", false
	}

	content, exists := workspace.Get(path)
	if !exists {
		return "", "", false
	}

	lines := strings.Split(content, "\n")
	startLine := 1
	endLine := len(lines)

	if value, exists := input["start_line"]; exists {
		number, ok := value.(float64)
		if !ok || number < 1 || number != float64(int(number)) {
			return "", "", false
		}
		startLine = int(number)
	}
	if startLine > len(lines) {
		return "", "", false
	}

	if value, exists := input["line_count"]; exists {
		number, ok := value.(float64)
		if !ok || number < 1 || number != float64(int(number)) {
			return "", "", false
		}
		count := int(number)
		if startLine+count-1 < endLine {
			endLine = startLine + count - 1
		}
	}

	key := fmt.Sprintf("%s:%d:%d", path, startLine, endLine)
	return key, hashFor(content), true
}

func estimateMessageTokens(messages []Message) int {
	characters := 0
	for _, message := range messages {
		characters += len(message.Role)
		for _, block := range message.Content {
			characters += len(block.Kind)
			characters += len(block.Text)

			if block.ToolUse != nil {
				characters += len(block.ToolUse.ID)
				characters += len(block.ToolUse.Name)
				encoded, _ := json.Marshal(block.ToolUse.Input)
				characters += len(encoded)
			}
			if block.ToolResult != nil {
				characters += len(block.ToolResult.ToolUseID)
				characters += len(block.ToolResult.Output)
			}
		}
	}

	if characters == 0 {
		return 0
	}
	return (characters + contextCharsPerToken - 1) / contextCharsPerToken
}

func summarizeMessages(messages []Message) string {
	var summary strings.Builder

	for _, message := range messages {
		for _, block := range message.Content {
			switch {
			case block.Kind == "text" && block.Text != "":
				fmt.Fprintf(
					&summary,
					"- %s: %s\n",
					message.Role,
					compactText(block.Text, 500),
				)

			case block.Kind == "tool_use" && block.ToolUse != nil:
				fmt.Fprintf(
					&summary,
					"- tool %s: %s\n",
					block.ToolUse.Name,
					summarizeInput(
						block.ToolUse.Name,
						block.ToolUse.Input,
					),
				)

			case block.Kind == "tool_result" && block.ToolResult != nil:
				fmt.Fprintf(
					&summary,
					"- tool result: %s\n",
					compactText(block.ToolResult.Output, 300),
				)
			}

			if summary.Len() >= maxContextSummaryChars {
				return compactText(
					summary.String(),
					maxContextSummaryChars,
				)
			}
		}
	}

	if summary.Len() == 0 {
		return "No earlier context."
	}
	return summary.String()
}

// enforceContextBudget progressively reduces retained content until the
// estimated context fits within the configured token budget.
func enforceContextBudget(
	messages []Message,
	budget int,
) []Message {
	blockLimit := maxRetainedBlockChars

	for estimateMessageTokens(messages) > budget &&
		blockLimit > 1 {
		blockLimit /= 2
		messages = compactRetainedMessages(
			messages,
			blockLimit,
		)
	}

	return messages
}

func compactRetainedMessages(
	messages []Message,
	blockLimit int,
) []Message {
	out := make([]Message, 0, len(messages))

	for _, message := range messages {
		copyMessage := Message{
			Role: message.Role,
			Content: make(
				[]ContentBlock,
				0,
				len(message.Content),
			),
		}

		for _, block := range message.Content {
			copyBlock := block
			copyBlock.Text = compactText(
				block.Text,
				blockLimit,
			)

			if block.ToolUse != nil {
				copyToolUse := *block.ToolUse
				copyToolUse.Input = compactToolInput(
					block.ToolUse.Input,
					blockLimit,
				)
				copyBlock.ToolUse = &copyToolUse
			}

			if block.ToolResult != nil {
				copyResult := *block.ToolResult
				copyResult.Output = compactText(
					block.ToolResult.Output,
					blockLimit,
				)
				copyBlock.ToolResult = &copyResult
			}

			copyMessage.Content = append(
				copyMessage.Content,
				copyBlock,
			)
		}

		out = append(out, copyMessage)
	}

	return out
}

func compactToolInput(
	input map[string]any,
	blockLimit int,
) map[string]any {
	if input == nil {
		return nil
	}

	out := make(map[string]any, len(input))
	for key, value := range input {
		if text, ok := value.(string); ok {
			out[key] = compactText(text, blockLimit)
			continue
		}
		out[key] = value
	}
	return out
}

func compactText(text string, limit int) string {
	const marker = "\n[truncated]"

	if limit <= 0 || len(text) <= limit {
		return text
	}
	if limit <= len(marker) {
		return text[:limit]
	}
	return text[:limit-len(marker)] + marker
}
