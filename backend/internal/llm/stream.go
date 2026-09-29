package llm

import "context"

// LLMStreamClient is the engine-neutral interface used by the agent loop.
// Implementations translate model responses into NormalizedChunk values
// so chat_v2.go remains independent of the selected local model.
type LLMStreamClient interface {
	// StreamTurn opens a streaming completion for one turn and returns a
	// reader the caller drains in order. Closing the reader (or ctx
	// cancellation) aborts the upstream HTTP request.
	StreamTurn(ctx context.Context, req TurnRequest) (StreamReader, error)
}

// TurnRequest is what the agent loop hands to the stream client per turn.
// Messages contains the running conversation history. Tools contains the
// functions the model may call during this turn.
type TurnRequest struct {
	SystemPrompt string
	Messages     []Message
	Tools        []ToolSpec
	// MaxOutputTokens caps the streamed completion; zero = engine default.
	MaxOutputTokens int
}

// Message is the engine-neutral chat history row. Content is a list of
// content blocks; each block is one of text / tool_use / tool_result.
type Message struct {
	Role    string         // "user" | "assistant" | "tool"
	Content []ContentBlock // see ContentBlock.Kind
}

// ContentBlock is one piece of a message body. Exactly one of the
// pointer fields is non-nil according to Kind.
type ContentBlock struct {
	Kind       string             // "text" | "tool_use" | "tool_result"
	Text       string             // for "text"
	ToolUse    *ContentToolUse    // for "tool_use"
	ToolResult *ContentToolResult // for "tool_result"
}

type ContentToolUse struct {
	ID    string
	Name  string
	Input map[string]any
}

type ContentToolResult struct {
	ToolUseID string
	Output    string
	IsError   bool
}

// ToolSpec is the engine-neutral tool advertisement passed to the LLM.
// InputSchema is a JSON-Schema dict; adapters translate it to vendor format.
type ToolSpec struct {
	Name        string
	Description string
	InputSchema map[string]any
}

// StreamReader is a one-shot, sequentially-consumed cursor over
// normalized chunks. Implementations must surface every chunk in
// arrival order. Close is mandatory and idempotent.
type StreamReader interface {
	// Next advances. Returns ok=false at end of stream.
	Next(ctx context.Context) (chunk NormalizedChunk, ok bool, err error)
	// Close releases any underlying HTTP resources.
	Close() error
}

// NormalizedChunk is the union of streaming events the main loop cares
// about. Kind tells you which field to read.
type NormalizedChunk struct {
	Kind string // "text_delta" | "tool_use_start" | "tool_use_input_delta" | "tool_use_stop" | "message_stop" | "usage"
	// text_delta
	Text string
	// tool_use_start
	ToolUseID   string
	ToolUseName string
	// tool_use_input_delta: partial JSON for ToolUseID
	InputJSONDelta string
	// message_stop
	StopReason string // "end_turn" | "tool_use" | "max_tokens" | ...
	// usage (emitted once at message_stop or end)
	InputTokens         int
	OutputTokens        int
	CacheCreationTokens int
	CacheReadTokens     int
}
