package tips

import (
	"context"
	"strings"
	"testing"

	"codritium/backend/internal/llm"
)

type tutorTestStream struct {
	request llm.TurnRequest
	chunks  []llm.NormalizedChunk
}

func (s *tutorTestStream) StreamTurn(
	ctx context.Context,
	req llm.TurnRequest,
) (llm.StreamReader, error) {
	s.request = req
	return &tutorTestReader{
		chunks: append([]llm.NormalizedChunk(nil), s.chunks...),
	}, nil
}

type tutorTestReader struct {
	chunks []llm.NormalizedChunk
	index  int
}

func (r *tutorTestReader) Next(
	ctx context.Context,
) (llm.NormalizedChunk, bool, error) {
	if err := ctx.Err(); err != nil {
		return llm.NormalizedChunk{}, false, err
	}
	if r.index >= len(r.chunks) {
		return llm.NormalizedChunk{}, false, nil
	}

	chunk := r.chunks[r.index]
	r.index++
	return chunk, true, nil
}

func (r *tutorTestReader) Close() error {
	return nil
}

func TestAgentUsesConfiguredLocalModelStream(t *testing.T) {
	stream := &tutorTestStream{
		chunks: []llm.NormalizedChunk{
			{
				Kind: "text_delta",
				Text: "What have you ",
			},
			{
				Kind: "text_delta",
				Text: "tried so far?",
			},
			{
				Kind:       "message_stop",
				StopReason: "end_turn",
			},
		},
	}
	agent := New(stream, "qwen3:8b")

	response, err := agent.Ask(context.Background(), Request{
		Mode:        ModePractice,
		SoulPrebake: "Guide the candidate with questions.",
		Conversation: []Turn{
			{
				Role: "user",
				Text: "I am stuck.",
			},
		},
	})
	if err != nil {
		t.Fatalf("Ask() error = %v", err)
	}

	if response.Text != "What have you tried so far?" {
		t.Fatalf("response text = %q", response.Text)
	}
	if response.Model != "qwen3:8b" {
		t.Fatalf(
			"response model = %q, want qwen3:8b",
			response.Model,
		)
	}
	if len(stream.request.Messages) != 1 {
		t.Fatalf(
			"messages = %d, want 1",
			len(stream.request.Messages),
		)
	}
	if stream.request.Messages[0].Role != "user" {
		t.Fatalf(
			"message role = %q, want user",
			stream.request.Messages[0].Role,
		)
	}
	if stream.request.MaxOutputTokens != 4096 {
		t.Fatalf(
			"max output tokens = %d, want 4096",
			stream.request.MaxOutputTokens,
		)
	}
	if stream.request.SystemPrompt == "" {
		t.Fatal("system prompt is empty")
	}
}

func TestAgentRejectsNonPracticeMode(t *testing.T) {
	agent := &Agent{}

	for _, mode := range []Mode{
		ModeSimulator,
		ModeReply,
		Mode(""),
	} {
		_, err := agent.Ask(context.Background(), Request{
			Mode: mode,
			Conversation: []Turn{
				{
					Role: "user",
					Text: "hi",
				},
			},
		})
		if err == nil {
			t.Fatalf(
				"expected mode %q to be rejected",
				mode,
			)
		}
		if !strings.Contains(err.Error(), "mode") {
			t.Fatalf(
				"expected mode error for %q, got %v",
				mode,
				err,
			)
		}
	}
}

func TestAgentRejectsEmptyConversation(t *testing.T) {
	agent := &Agent{}

	_, err := agent.Ask(context.Background(), Request{
		Mode: ModePractice,
	})
	if err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf(
			"expected empty conversation error, got %v",
			err,
		)
	}
}

func TestAgentRejectsNilStream(t *testing.T) {
	var agent *Agent

	_, err := agent.Ask(context.Background(), Request{
		Mode: ModePractice,
		Conversation: []Turn{
			{
				Role: "user",
				Text: "hi",
			},
		},
	})
	if err == nil ||
		!strings.Contains(err.Error(), "not configured") {
		t.Fatalf(
			"expected stream configuration error, got %v",
			err,
		)
	}
}
