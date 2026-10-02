package llm

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

type OllamaStream struct {
	BaseURL string
	Model   string
	Client  *http.Client
}

func NewOllamaStream(baseURL, model string, timeout time.Duration) *OllamaStream {
	return &OllamaStream{
		BaseURL: strings.TrimRight(baseURL, "/"),
		Model:   model,
		Client:  &http.Client{Timeout: timeout},
	}
}

const ollamaContextWindow = 8192

type ollamaStreamRequest struct {
	Model    string                `json:"model"`
	Messages []ollamaStreamMessage `json:"messages"`
	Tools    []ollamaStreamTool    `json:"tools,omitempty"`
	Stream   bool                  `json:"stream"`
	Think    bool                  `json:"think"`
	Options  ollamaStreamOptions   `json:"options"`
}

type ollamaStreamOptions struct {
	Temperature float64 `json:"temperature"`
	NumPredict  int     `json:"num_predict,omitempty"`
	NumCtx      int     `json:"num_ctx,omitempty"`
}

type ollamaStreamMessage struct {
	Role      string                 `json:"role"`
	Content   string                 `json:"content,omitempty"`
	ToolCalls []ollamaStreamToolCall `json:"tool_calls,omitempty"`
	ToolName  string                 `json:"tool_name,omitempty"`
}

type ollamaStreamTool struct {
	Type     string               `json:"type"`
	Function ollamaStreamFunction `json:"function"`
}

type ollamaStreamToolCall struct {
	Type     string               `json:"type,omitempty"`
	Function ollamaStreamFunction `json:"function"`
}

type ollamaStreamFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Arguments   map[string]any `json:"arguments,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

type ollamaStreamResponse struct {
	Message         ollamaStreamMessage `json:"message"`
	Done            bool                `json:"done"`
	DoneReason      string              `json:"done_reason"`
	PromptEvalCount int                 `json:"prompt_eval_count"`
	EvalCount       int                 `json:"eval_count"`
	Error           string              `json:"error"`
}

func (o *OllamaStream) StreamTurn(ctx context.Context, req TurnRequest) (StreamReader, error) {
	if strings.TrimSpace(o.BaseURL) == "" {
		return nil, fmt.Errorf("ollama stream: base URL is required")
	}
	if strings.TrimSpace(o.Model) == "" {
		return nil, fmt.Errorf("ollama stream: model is required")
	}
	if o.Client == nil {
		return nil, fmt.Errorf("ollama stream: HTTP client is not configured")
	}

	messages, err := toOllamaMessages(req.SystemPrompt, req.Messages)
	if err != nil {
		return nil, fmt.Errorf("ollama stream: encode messages: %w", err)
	}

	body := ollamaStreamRequest{
		Model:    o.Model,
		Messages: messages,
		Tools:    toOllamaTools(req.Tools),
		Stream:   true,
		Think:    false,
		Options: ollamaStreamOptions{
			Temperature: 0.2,
			NumPredict:  req.MaxOutputTokens,
			NumCtx:      ollamaContextWindow,
		},
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("ollama stream: encode request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		o.BaseURL+"/api/chat",
		bytes.NewReader(payload),
	)
	if err != nil {
		return nil, fmt.Errorf("ollama stream: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := o.Client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("ollama stream: request: %w", err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		defer resp.Body.Close()
		detail, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
		return nil, fmt.Errorf(
			"ollama stream: HTTP %d: %s",
			resp.StatusCode,
			strings.TrimSpace(string(detail)),
		)
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)

	toolIDPrefix, err := newToolIDPrefix()
	if err != nil {
		resp.Body.Close()
		return nil, err
	}

	return &ollamaStreamReader{
		body:         resp.Body,
		scanner:      scanner,
		toolIDPrefix: toolIDPrefix,
	}, nil
}

func newToolIDPrefix() (string, error) {
	var value [12]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("generate tool ID: %w", err)
	}
	return hex.EncodeToString(value[:]), nil
}

func toOllamaMessages(systemPrompt string, messages []Message) ([]ollamaStreamMessage, error) {
	out := make([]ollamaStreamMessage, 0, len(messages)+1)
	if systemPrompt != "" {
		out = append(out, ollamaStreamMessage{
			Role:    "system",
			Content: systemPrompt,
		})
	}

	toolNames := make(map[string]string)
	for _, message := range messages {
		for _, block := range message.Content {
			if block.Kind == "tool_use" && block.ToolUse != nil {
				toolNames[block.ToolUse.ID] = block.ToolUse.Name
			}
		}
	}

	for _, message := range messages {
		switch message.Role {
		case "user":
			if content := textFromBlocks(message.Content); content != "" {
				out = append(out, ollamaStreamMessage{
					Role:    "user",
					Content: content,
				})
			}

		case "assistant":
			encoded := ollamaStreamMessage{
				Role:    "assistant",
				Content: textFromBlocks(message.Content),
			}
			for _, block := range message.Content {
				if block.Kind != "tool_use" || block.ToolUse == nil {
					continue
				}
				encoded.ToolCalls = append(encoded.ToolCalls, ollamaStreamToolCall{
					Type: "function",
					Function: ollamaStreamFunction{
						Name:      block.ToolUse.Name,
						Arguments: block.ToolUse.Input,
					},
				})
			}
			if encoded.Content != "" || len(encoded.ToolCalls) > 0 {
				out = append(out, encoded)
			}

		case "tool":
			for _, block := range message.Content {
				if block.Kind != "tool_result" || block.ToolResult == nil {
					continue
				}

				toolName := toolNames[block.ToolResult.ToolUseID]
				if toolName == "" {
					return nil, fmt.Errorf(
						"tool result %q has no matching tool call",
						block.ToolResult.ToolUseID,
					)
				}

				out = append(out, ollamaStreamMessage{
					Role:     "tool",
					ToolName: toolName,
					Content:  block.ToolResult.Output,
				})
			}

		default:
			return nil, fmt.Errorf("unsupported message role %q", message.Role)
		}
	}

	return out, nil
}

func textFromBlocks(blocks []ContentBlock) string {
	var text strings.Builder
	for _, block := range blocks {
		if block.Kind == "text" {
			text.WriteString(block.Text)
		}
	}
	return text.String()
}

func toOllamaTools(tools []ToolSpec) []ollamaStreamTool {
	out := make([]ollamaStreamTool, 0, len(tools))
	for _, tool := range tools {
		out = append(out, ollamaStreamTool{
			Type: "function",
			Function: ollamaStreamFunction{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.InputSchema,
			},
		})
	}
	return out
}

type ollamaStreamReader struct {
	body         io.ReadCloser
	scanner      *bufio.Scanner
	queue        []NormalizedChunk
	done         bool
	sawToolUse   bool
	toolIDPrefix string
	toolIndex    int
	closeOnce    sync.Once
}

func (r *ollamaStreamReader) Next(ctx context.Context) (NormalizedChunk, bool, error) {
	for len(r.queue) == 0 {
		if err := ctx.Err(); err != nil {
			return NormalizedChunk{}, false, err
		}
		if r.done {
			return NormalizedChunk{}, false, nil
		}
		if !r.scanner.Scan() {
			if err := r.scanner.Err(); err != nil {
				return NormalizedChunk{}, false, fmt.Errorf(
					"ollama stream: read response: %w",
					err,
				)
			}
			return NormalizedChunk{}, false, fmt.Errorf(
				"ollama stream: response ended before done",
			)
		}

		var response ollamaStreamResponse
		if err := json.Unmarshal(r.scanner.Bytes(), &response); err != nil {
			return NormalizedChunk{}, false, fmt.Errorf(
				"ollama stream: decode response: %w",
				err,
			)
		}
		if response.Error != "" {
			return NormalizedChunk{}, false, fmt.Errorf(
				"ollama stream: %s",
				response.Error,
			)
		}

		r.enqueue(response)
	}

	chunk := r.queue[0]
	r.queue = r.queue[1:]
	return chunk, true, nil
}

func (r *ollamaStreamReader) Close() error {
	var err error
	r.closeOnce.Do(func() {
		err = r.body.Close()
	})
	return err
}

func (r *ollamaStreamReader) enqueue(response ollamaStreamResponse) {
	if response.Message.Content != "" {
		r.queue = append(r.queue, NormalizedChunk{
			Kind: "text_delta",
			Text: response.Message.Content,
		})
	}

	for _, call := range response.Message.ToolCalls {
		r.sawToolUse = true
		r.toolIndex++

		toolUseID := fmt.Sprintf(
			"tu-ollama-%s-%d",
			r.toolIDPrefix,
			r.toolIndex,
		)
		arguments := call.Function.Arguments
		if arguments == nil {
			arguments = map[string]any{}
		}
		input, _ := json.Marshal(arguments)

		r.queue = append(
			r.queue,
			NormalizedChunk{
				Kind:        "tool_use_start",
				ToolUseID:   toolUseID,
				ToolUseName: call.Function.Name,
			},
			NormalizedChunk{
				Kind:           "tool_use_input_delta",
				ToolUseID:      toolUseID,
				InputJSONDelta: string(input),
			},
			NormalizedChunk{
				Kind:      "tool_use_stop",
				ToolUseID: toolUseID,
			},
		)
	}

	if !response.Done {
		return
	}

	r.done = true
	stopReason := response.DoneReason
	if r.sawToolUse {
		stopReason = "tool_use"
	} else if stopReason == "" || stopReason == "stop" {
		stopReason = "end_turn"
	}

	r.queue = append(
		r.queue,
		NormalizedChunk{
			Kind:         "usage",
			InputTokens:  response.PromptEvalCount,
			OutputTokens: response.EvalCount,
		},
		NormalizedChunk{
			Kind:       "message_stop",
			StopReason: stopReason,
		},
	)
}
