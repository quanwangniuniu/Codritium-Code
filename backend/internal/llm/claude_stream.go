package llm

import (
	"context"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"

	anthClient "codritium/backend/internal/anthropic"
)

// ClaudeStream adapts the Anthropic SDK streaming surface to the
// engine-neutral LLMStreamClient. v0.8 ships with this preserved but
// inactive (the default chat engine is Gemini per decision_log D1);
// flipping `CHAT_ENGINE=anthropic` re-enables this path.
//
// Anthropic streams blocks: content_block_start / content_block_delta
// (text_delta or input_json_delta) / content_block_stop / message_delta
// (carries usage) / message_stop. The translation to NormalizedChunk
// mirrors GeminiStream's so drainStream() handles both uniformly.
type ClaudeStream struct {
	Client *anthClient.Client
	Model  string // empty → ModelSonnet
}

func NewClaudeStream(client *anthClient.Client) *ClaudeStream {
	return &ClaudeStream{Client: client, Model: anthClient.ModelSonnet}
}

func (c *ClaudeStream) StreamTurn(ctx context.Context, req TurnRequest) (StreamReader, error) {
	if c.Client == nil {
		return nil, fmt.Errorf("claude stream: client not configured")
	}
	model := c.Model
	if model == "" {
		model = anthClient.ModelSonnet
	}

	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(model),
		MaxTokens: 4096,
	}
	if req.SystemPrompt != "" {
		params.System = []anthropic.TextBlockParam{{Text: req.SystemPrompt}}
	}

	// Translate messages
	msgs, err := toAnthropicMessages(req.Messages)
	if err != nil {
		return nil, fmt.Errorf("claude stream: encode messages: %w", err)
	}
	params.Messages = msgs

	if len(req.Tools) > 0 {
		params.Tools = toAnthropicTools(req.Tools)
	}

	stream := c.Client.Underlying().Messages.NewStreaming(ctx, params)
	return &claudeReader{stream: stream}, nil
}

// toAnthropicMessages is intentionally minimal in V0; the inactive
// path doesn't need full fidelity until CHAT_ENGINE=anthropic is
// flipped. We carry text + tool_use + tool_result blocks; everything
// else collapses into text.
func toAnthropicMessages(msgs []Message) ([]anthropic.MessageParam, error) {
	out := make([]anthropic.MessageParam, 0, len(msgs))
	for _, m := range msgs {
		switch m.Role {
		case "user":
			text := firstText(m.Content)
			out = append(out, anthropic.NewUserMessage(anthropic.NewTextBlock(text)))
		case "assistant":
			text := firstText(m.Content)
			out = append(out, anthropic.NewAssistantMessage(anthropic.NewTextBlock(text)))
		case "tool":
			// Tool results map to a user message containing tool_result blocks
			// in the Anthropic API. For V0 we fold a stringified payload into
			// a synthetic user message so the model still sees the result.
			for _, b := range m.Content {
				if b.ToolResult == nil {
					continue
				}
				out = append(out, anthropic.NewUserMessage(
					anthropic.NewTextBlock("[tool_result for "+b.ToolResult.ToolUseID+"] "+b.ToolResult.Output),
				))
			}
		}
	}
	return out, nil
}

func firstText(blocks []ContentBlock) string {
	for _, b := range blocks {
		if b.Kind == "text" {
			return b.Text
		}
	}
	return ""
}

func toAnthropicTools(_ []ToolSpec) []anthropic.ToolUnionParam {
	// V0 stub: the inactive Anthropic path doesn't yet wire tools through
	// because the candidate-agent will be exercised via Gemini. Flip when
	// re-activating Claude (CHAT_ENGINE=anthropic) and write the schema
	// translator then.
	return nil
}

// claudeReader translates Anthropic stream events into NormalizedChunk.
// Anthropic's content_block_delta for tool_use carries partial JSON via
// the input_json_delta field, which is exactly what drainStream's
// accumulator expects — no buffering required here.
type claudeReader struct {
	stream interface {
		Next() bool
		Current() anthropic.MessageStreamEventUnion
		Err() error
		Close() error
	}
	queue        []NormalizedChunk
	closed       bool
	usageEmitted bool
	stopEmitted  bool

	// per-block bookkeeping while a tool_use content_block is open
	curToolID   string
	curToolName string
}

func (r *claudeReader) Next(ctx context.Context) (NormalizedChunk, bool, error) {
	for len(r.queue) == 0 {
		if err := ctx.Err(); err != nil {
			return NormalizedChunk{}, false, err
		}
		if !r.stream.Next() {
			if err := r.stream.Err(); err != nil {
				return NormalizedChunk{}, false, err
			}
			// stream ended: emit trailing usage / message_stop if missing
			if !r.usageEmitted {
				r.usageEmitted = true
				r.queue = append(r.queue, NormalizedChunk{Kind: "usage"})
			}
			if !r.stopEmitted {
				r.stopEmitted = true
				r.queue = append(r.queue, NormalizedChunk{Kind: "message_stop", StopReason: "end_turn"})
				continue
			}
			return NormalizedChunk{}, false, nil
		}
		r.translate(r.stream.Current())
	}
	c := r.queue[0]
	r.queue = r.queue[1:]
	return c, true, nil
}

func (r *claudeReader) Close() error {
	if r.closed {
		return nil
	}
	r.closed = true
	return r.stream.Close()
}

func (r *claudeReader) translate(ev anthropic.MessageStreamEventUnion) {
	switch ev.Type {
	case "content_block_start":
		// ev.ContentBlock can be a text or tool_use block
		cb := ev.ContentBlock
		if cb.Type == "tool_use" {
			r.curToolID = cb.ID
			r.curToolName = cb.Name
			r.queue = append(r.queue, NormalizedChunk{
				Kind:        "tool_use_start",
				ToolUseID:   cb.ID,
				ToolUseName: cb.Name,
			})
		}
	case "content_block_delta":
		d := ev.Delta
		switch d.Type {
		case "text_delta":
			if d.Text != "" {
				r.queue = append(r.queue, NormalizedChunk{Kind: "text_delta", Text: d.Text})
			}
		case "input_json_delta":
			if r.curToolID != "" {
				r.queue = append(r.queue, NormalizedChunk{
					Kind:           "tool_use_input_delta",
					ToolUseID:      r.curToolID,
					InputJSONDelta: d.PartialJSON,
				})
			}
		}
	case "content_block_stop":
		if r.curToolID != "" {
			r.queue = append(r.queue, NormalizedChunk{
				Kind:      "tool_use_stop",
				ToolUseID: r.curToolID,
			})
			r.curToolID = ""
			r.curToolName = ""
		}
	case "message_delta":
		// Usage info often comes here
		if ev.Usage.OutputTokens > 0 || ev.Usage.InputTokens > 0 {
			r.usageEmitted = true
			r.queue = append(r.queue, NormalizedChunk{
				Kind:         "usage",
				InputTokens:  int(ev.Usage.InputTokens),
				OutputTokens: int(ev.Usage.OutputTokens),
			})
		}
		if reason := string(ev.Delta.StopReason); reason != "" && !r.stopEmitted {
			r.stopEmitted = true
			r.queue = append(r.queue, NormalizedChunk{Kind: "message_stop", StopReason: reason})
		}
	case "message_stop":
		if !r.stopEmitted {
			r.stopEmitted = true
			r.queue = append(r.queue, NormalizedChunk{Kind: "message_stop", StopReason: "end_turn"})
		}
	}
}
