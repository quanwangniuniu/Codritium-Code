package tips

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"codritium/backend/internal/llm"
)

const defaultModel = "qwen3:8b"

// Mode identifies the session stage in which the tutor is used.
// Tutor responses are available only during practice.
type Mode string

const (
	ModePractice  Mode = "practice"
	ModeSimulator Mode = "simulator"
	ModeReply     Mode = "reply"
)

// Turn is one user or tutor message in the conversation history.
type Turn struct {
	Role string
	Text string
}

// Request contains the problem guidance, conversation history, workspace
// snapshot, and candidate-agent activity supplied to the tutor.
type Request struct {
	Mode         Mode
	SoulPrebake  string
	AgentSummary string
	Conversation []Turn
	FileContents map[string]string
}

// StreamChunk contains one text fragment or the terminal state of a tutor response.
type StreamChunk struct {
	Delta string
	Done  bool
	Err   error
}

// Response contains the completed tutor reply and configured model name.
type Response struct {
	Text  string
	Model string
}

// Agent uses the shared local model stream to produce Socratic tutor responses.
type Agent struct {
	Stream llm.LLMStreamClient
	Model  string
}

// New creates a tutor agent backed by the configured local Ollama model.
func New(stream llm.LLMStreamClient, model string) *Agent {
	if model == "" {
		model = defaultModel
	}
	return &Agent{
		Stream: stream,
		Model:  model,
	}
}

// Ask collects one streamed tutor response and returns it as complete text.
func (a *Agent) Ask(ctx context.Context, req Request) (Response, error) {
	stream, err := a.AskStream(ctx, req)
	if err != nil {
		return Response{}, err
	}

	var text strings.Builder
	for chunk := range stream {
		if chunk.Err != nil {
			return Response{}, chunk.Err
		}
		text.WriteString(chunk.Delta)
	}

	result := text.String()
	if strings.TrimSpace(result) == "" {
		return Response{}, errors.New("tips agent: empty response from Ollama")
	}

	model := a.Model
	if model == "" {
		model = defaultModel
	}
	return Response{
		Text:  result,
		Model: model,
	}, nil
}

// AskStream starts one streaming tutor turn. Only practice mode is allowed.
func (a *Agent) AskStream(
	ctx context.Context,
	req Request,
) (<-chan StreamChunk, error) {
	if req.Mode != ModePractice {
		return nil, fmt.Errorf(
			"tips agent: mode %q not allowed",
			req.Mode,
		)
	}
	if len(req.Conversation) == 0 {
		return nil, errors.New("tips agent: empty conversation")
	}
	if a == nil || a.Stream == nil {
		return nil, errors.New("tips agent: Ollama stream not configured")
	}

	reader, err := a.Stream.StreamTurn(ctx, llm.TurnRequest{
		SystemPrompt:    BuildTipsPrompt(req.SoulPrebake, req.AgentSummary, req.FileContents),
		Messages:        toLLMMessages(req.Conversation),
		MaxOutputTokens: 4096,
	})
	if err != nil {
		return nil, fmt.Errorf("tips agent: Ollama request: %w", err)
	}

	out := make(chan StreamChunk)
	go func() {
		defer close(out)
		defer reader.Close()

		emitted := false
		for {
			chunk, ok, err := reader.Next(ctx)
			if err != nil {
				sendStreamChunk(ctx, out, StreamChunk{
					Err: fmt.Errorf("tips stream: %w", err),
				})
				return
			}
			if !ok {
				break
			}
			if chunk.Kind != "text_delta" || chunk.Text == "" {
				continue
			}

			emitted = true
			if !sendStreamChunk(ctx, out, StreamChunk{
				Delta: chunk.Text,
			}) {
				return
			}
		}

		if !emitted {
			sendStreamChunk(ctx, out, StreamChunk{
				Err: errors.New("tips stream: empty response from Ollama"),
			})
			return
		}

		sendStreamChunk(ctx, out, StreamChunk{Done: true})
	}()

	return out, nil
}

// toLLMMessages converts persisted tutor turns into the shared LLM message format.
func toLLMMessages(turns []Turn) []llm.Message {
	out := make([]llm.Message, 0, len(turns))
	for _, turn := range turns {
		role := "user"
		if turn.Role == "model" {
			role = "assistant"
		}

		out = append(out, llm.Message{
			Role: role,
			Content: []llm.ContentBlock{
				{
					Kind: "text",
					Text: turn.Text,
				},
			},
		})
	}
	return out
}

func sendStreamChunk(
	ctx context.Context,
	out chan<- StreamChunk,
	chunk StreamChunk,
) bool {
	select {
	case out <- chunk:
		return true
	case <-ctx.Done():
		return false
	}
}
