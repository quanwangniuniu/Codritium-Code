package tips

import (
	"context"
	"errors"
	"fmt"

	"google.golang.org/genai"
)

// classifierModel is the lightweight Gemini variant the C2 stage calls.
// Flash-Lite is the right cost/latency target for a single-shot YES/NO/
// UNCLEAR verdict; the heavier flash model is reserved for tutor turns.
const classifierModel = "gemini-2.5-flash-lite"

// GeminiClassifier is the Classifier implementation that asks Gemini
// Flash-Lite for a verdict on a regex-flagged conversation excerpt.
type GeminiClassifier struct {
	Client *genai.Client
	Model  string
}

// NewGeminiClassifier wires a Gemini client into the Classifier
// interface. Model defaults to gemini-2.5-flash-lite when blank.
func NewGeminiClassifier(client *genai.Client, model string) *GeminiClassifier {
	if model == "" {
		model = classifierModel
	}
	return &GeminiClassifier{Client: client, Model: model}
}

// Classify runs a non-streaming generation against the classifier model.
// The prompt is the full text the filter assembled (DefaultClassifyPrompt
// %s-substituted), so we send it as a single user content block with no
// system instruction — the verdict format is already baked into the prompt.
func (g *GeminiClassifier) Classify(ctx context.Context, prompt string) (string, error) {
	if g == nil || g.Client == nil {
		return "", errors.New("gemini classifier: client not configured")
	}
	model := g.Model
	if model == "" {
		model = classifierModel
	}
	contents := []*genai.Content{
		genai.NewContentFromText(prompt, genai.RoleUser),
	}
	cfg := &genai.GenerateContentConfig{
		Temperature:     genai.Ptr[float32](0),
		MaxOutputTokens: 8,
	}
	resp, err := g.Client.Models.GenerateContent(ctx, model, contents, cfg)
	if err != nil {
		return "", fmt.Errorf("gemini classifier: %w", err)
	}
	return extractText(resp), nil
}
