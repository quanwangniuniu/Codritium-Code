package tips

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"codritium/backend/internal/llm"
)

type OllamaClassifier struct {
	Stream llm.LLMStreamClient
}

func NewOllamaClassifier(
	stream llm.LLMStreamClient,
) *OllamaClassifier {
	return &OllamaClassifier{Stream: stream}
}

func (c *OllamaClassifier) Classify(
	ctx context.Context,
	prompt string,
) (string, error) {
	if c == nil || c.Stream == nil {
		return "", errors.New(
			"Ollama classifier: stream not configured",
		)
	}

	reader, err := c.Stream.StreamTurn(ctx, llm.TurnRequest{
		Messages: []llm.Message{
			{
				Role: "user",
				Content: []llm.ContentBlock{
					{
						Kind: "text",
						Text: prompt,
					},
				},
			},
		},
		MaxOutputTokens: 8,
	})
	if err != nil {
		return "", fmt.Errorf(
			"Ollama classifier: request: %w",
			err,
		)
	}
	defer reader.Close()

	var result strings.Builder
	for {
		chunk, ok, err := reader.Next(ctx)
		if err != nil {
			return "", fmt.Errorf(
				"Ollama classifier: read: %w",
				err,
			)
		}
		if !ok {
			break
		}
		if chunk.Kind == "text_delta" {
			result.WriteString(chunk.Text)
		}
	}

	verdict := strings.TrimSpace(result.String())
	if verdict == "" {
		return "", errors.New(
			"Ollama classifier: empty response",
		)
	}
	return verdict, nil
}
