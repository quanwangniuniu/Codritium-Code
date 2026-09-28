package anthropic

import (
	"context"
	"errors"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

const ModelSonnet = "claude-sonnet-4-6"

type Client struct {
	c anthropic.Client
}

func New(apiKey string) *Client {
	return &Client{
		c: anthropic.NewClient(option.WithAPIKey(apiKey)),
	}
}

// Ping does a minimal Sonnet call to verify the key works.
// Returns the assistant's reply text.
func (a *Client) Ping(ctx context.Context) (string, error) {
	msg, err := a.c.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(ModelSonnet),
		MaxTokens: 64,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock("Say 'pong' in one word.")),
		},
	})
	if err != nil {
		return "", err
	}
	if len(msg.Content) == 0 {
		return "", errors.New("empty response")
	}
	return msg.Content[0].Text, nil
}

// Underlying returns the raw SDK client for advanced usage (streaming, etc.).
func (a *Client) Underlying() *anthropic.Client {
	return &a.c
}
