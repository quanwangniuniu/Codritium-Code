package grader

import (
	"context"
	"fmt"

	"google.golang.org/genai"

	anthClient "codritium/backend/internal/anthropic"
)

// Engine scores one submission on the rubric.
type Engine interface {
	Name() string
	Grade(ctx context.Context, input GraderInput) (*GraderResult, error)
}

// Clients holds the provider clients an engine may need; only the one the
// chosen engine uses must be set.
type Clients struct {
	Anthropic *anthClient.Client
	Gemini    *genai.Client
	Ollama    *OllamaClient
}

// NewEngine returns the engine named by GRADER_ENGINE. This is the one
// place engines are selected (the server and the grade CLI both call it).
func NewEngine(name string, c Clients) (Engine, error) {
	switch name {
	case "ollama":
		if c.Ollama == nil {
			return nil, fmt.Errorf("grader engine ollama: client not configured")
		}
		return ollamaEngine{c.Ollama}, nil
	case "gemini":
		if c.Gemini == nil {
			return nil, fmt.Errorf("grader engine gemini: GOOGLE_API_KEY not configured")
		}
		return geminiEngine{c.Gemini}, nil
	case "anthropic":
		if c.Anthropic == nil {
			return nil, fmt.Errorf("grader engine anthropic: ANTHROPIC_API_KEY not configured")
		}
		return anthropicEngine{c.Anthropic}, nil
	}
	return nil, fmt.Errorf("unknown grader engine %q (use ollama|gemini|anthropic)", name)
}

type ollamaEngine struct{ c *OllamaClient }

func (e ollamaEngine) Name() string { return "ollama" }
func (e ollamaEngine) Grade(ctx context.Context, in GraderInput) (*GraderResult, error) {
	return RunOllama(ctx, e.c, in)
}

type geminiEngine struct{ c *genai.Client }

func (e geminiEngine) Name() string { return "gemini" }
func (e geminiEngine) Grade(ctx context.Context, in GraderInput) (*GraderResult, error) {
	return RunGemini(ctx, e.c, in)
}

type anthropicEngine struct{ c *anthClient.Client }

func (e anthropicEngine) Name() string { return "anthropic" }
func (e anthropicEngine) Grade(ctx context.Context, in GraderInput) (*GraderResult, error) {
	return Run(ctx, e.c, in)
}
