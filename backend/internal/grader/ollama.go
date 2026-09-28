package grader

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"
)

// OllamaClient calls a locally running Ollama HTTP API.
type OllamaClient struct {
	baseURL    string
	model      string
	httpClient *http.Client
}

type ollamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaOptions struct {
	Temperature float64 `json:"temperature"`
	NumPredict  int     `json:"num_predict"`
}

type ollamaChatRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
	Think    bool            `json:"think"`
	Format   string          `json:"format"`
	Options  ollamaOptions   `json:"options"`
}

type ollamaChatResponse struct {
	Message ollamaMessage `json:"message"`
	Done    bool          `json:"done"`
	Error   string        `json:"error,omitempty"`
}

// NewOllamaClient creates a client for a locally running Ollama API.
func NewOllamaClient(
	baseURL string,
	model string,
	timeout time.Duration,
) *OllamaClient {
	return &OllamaClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		model:   model,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// Model returns the configured Ollama model name.
func (c *OllamaClient) Model() string {
	return c.model
}

func (c *OllamaClient) chat(
	ctx context.Context,
	systemPrompt string,
	userPrompt string,
) (string, error) {
	requestBody := ollamaChatRequest{
		Model: c.model,
		Messages: []ollamaMessage{
			{
				Role:    "system",
				Content: systemPrompt,
			},
			{
				Role:    "user",
				Content: userPrompt,
			},
		},
		Stream: false,
		Think:  false,
		Format: "json",
		Options: ollamaOptions{
			Temperature: 0,
			NumPredict:  2048,
		},
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("marshal Ollama request: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.baseURL+"/api/chat",
		bytes.NewReader(body),
	)
	if err != nil {
		return "", fmt.Errorf("create Ollama request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("call Ollama: %w", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return "", fmt.Errorf("read Ollama response: %w", err)
	}

	if resp.StatusCode < http.StatusOK ||
		resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf(
			"Ollama returned status %d: %s",
			resp.StatusCode,
			truncate(string(responseBody), 500),
		)
	}

	var result ollamaChatResponse
	if err := json.Unmarshal(responseBody, &result); err != nil {
		return "", fmt.Errorf("decode Ollama response: %w", err)
	}

	if result.Error != "" {
		return "", fmt.Errorf("Ollama error: %s", result.Error)
	}
	if !result.Done {
		return "", fmt.Errorf("Ollama response did not complete")
	}

	content := strings.TrimSpace(result.Message.Content)
	if content == "" {
		return "", fmt.Errorf("Ollama returned empty message content")
	}

	return content, nil
}

// RunOllama evaluates all five dimensions and four anti-patterns.
func RunOllama(
	ctx context.Context,
	client *OllamaClient,
	input GraderInput,
) (*GraderResult, error) {
	weights := DifficultyWeights(input.ProblemDifficulty)

	clean := SanitizeInput(input)
	clean = capInput(clean)

	results := make(map[string]DimensionScore, len(Dimensions))

	for _, dimension := range Dimensions {
		if !hasDimensionEvidence(clean, dimension.Name) {
			results[dimension.Name] = DimensionScore{
				Dimension: dimension.Name,
				Score:     nil,
				Reasoning: insufficientEvidenceReason(dimension.Name),
			}
			continue
		}

		score, err := gradeOneDimOllama(
			ctx,
			client,
			dimension,
			clean,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"grade dimension %s: %w",
				dimension.Name,
				err,
			)
		}

		results[dimension.Name] = score
	}

	antiPatterns, err := gradeAntiPatternsOllama(ctx, client, clean)
	if err != nil {
		return nil, fmt.Errorf("grade anti-patterns: %w", err)
	}

	finalScore, evaluated := aggregateScores(weights, results)

	return &GraderResult{
		DimensionScores: results,
		Weights:         weights,
		FinalScore:      finalScore,
		EvaluatedDims:   evaluated,
		AntiPatterns:    &antiPatterns,
		JudgeModel:      client.Model(),
	}, nil
}

func gradeOneDimOllama(
	ctx context.Context,
	client *OllamaClient,
	dimension DimensionSpec,
	input GraderInput,
) (DimensionScore, error) {
	text, err := client.chat(
		ctx,
		rubricSystem,
		buildUserPrompt(dimension, input),
	)
	if err != nil {
		return DimensionScore{}, err
	}

	score := parseDimensionJSON(text)
	score.Dimension = dimension.Name

	if score.Error != "" {
		return DimensionScore{}, fmt.Errorf("%s", score.Error)
	}
	if score.Reasoning == "" {
		return DimensionScore{}, fmt.Errorf("response has empty reasoning")
	}
	if score.Score == nil {
		return score, nil
	}
	if *score.Score < 1 || *score.Score > 5 {
		return DimensionScore{}, fmt.Errorf(
			"score %d is outside the allowed range 1-5",
			*score.Score,
		)
	}

	return score, nil
}

func hasDimensionEvidence(input GraderInput, dimension string) bool {
	switch dimension {
	case "correctness", "verification":
		return true
	case "ai_collaboration":
		return len(input.PromptHistory) > 0
	case "problem_decomposition", "communication":
		return len(input.PromptHistory) > 0 ||
			hasCandidateProcessArtifact(input)
	default:
		return false
	}
}

func hasCandidateProcessArtifact(input GraderInput) bool {
	for name, candidateContent := range input.CandidateFiles {
		base := strings.ToLower(filepath.Base(name))

		isProcessFile :=
			base == "agents.md" ||
				base == "plan.md" ||
				base == "scratch.md" ||
				base == "notes.md" ||
				strings.Contains(base, "plan") ||
				strings.Contains(base, "scratch")

		if !isProcessFile {
			continue
		}

		starterContent, existedInStarter := input.StarterFiles[name]
		if !existedInStarter || candidateContent != starterContent {
			if strings.TrimSpace(candidateContent) != "" {
				return true
			}
		}
	}

	return false
}

func insufficientEvidenceReason(dimension string) string {
	switch dimension {
	case "ai_collaboration":
		return "insufficient evidence: no AI interaction history"
	case "problem_decomposition":
		return "insufficient evidence: no prompt history or candidate-authored plan"
	case "communication":
		return "insufficient evidence: no narration or candidate-authored process artifact"
	default:
		return "insufficient evidence"
	}
}

func gradeAntiPatternsOllama(
	ctx context.Context,
	client *OllamaClient,
	input GraderInput,
) (AntiPatternBundle, error) {
	if len(input.PromptHistory) == 0 {
		notEvaluated := AntiPatternFlag{
			Evaluated: false,
			Triggered: false,
			Evidence:  "insufficient evidence: no AI interaction history",
		}

		return AntiPatternBundle{
			HandsOff:        notEvaluated,
			FeatureMarathon: notEvaluated,
			AIShowcase:      notEvaluated,
			NotThinking:     notEvaluated,
		}, nil
	}

	historyJSON, err := json.Marshal(input.PromptHistory)
	if err != nil {
		return AntiPatternBundle{}, fmt.Errorf(
			"marshal prompt history: %w",
			err,
		)
	}

	prompt := fmt.Sprintf(`Evaluate the candidate's process for four anti-patterns.

PROMPT HISTORY:
%s

Return exactly one JSON object with these four keys:
{
  "hands_off": {
    "evaluated": true,
    "triggered": <boolean>,
    "evidence": "<brief evidence or empty string>"
  },
  "feature_marathon": {
    "evaluated": true,
    "triggered": <boolean>,
    "evidence": "<brief evidence or empty string>"
  },
  "ai_showcase": {
    "evaluated": true,
    "triggered": <boolean>,
    "evidence": "<brief evidence or empty string>"
  },
  "not_thinking": {
    "evaluated": true,
    "triggered": <boolean>,
    "evidence": "<brief evidence or empty string>"
  }
}

Definitions:
- hands_off: delegates the task to AI and accepts output without review, correction, or verification.
- feature_marathon: repeatedly asks for more implementation without reviewing or validating earlier work.
- ai_showcase: focuses on demonstrating AI capabilities instead of solving the stated problem.
- not_thinking: gives away planning and judgment with instructions such as "just fix it" or "do whatever".

Rules:
- Use only evidence present in the prompt history.
- Do not infer an anti-pattern from missing evidence alone.
- Because prompt history is present, set evaluated to true for every pattern.
- If a pattern is not supported, set triggered to false and evidence to an empty string.
- Return JSON only.`, string(historyJSON))

	text, err := client.chat(
		ctx,
		"You are an evaluator detecting coding-interview anti-patterns. Return valid JSON only.",
		prompt,
	)
	if err != nil {
		return AntiPatternBundle{}, err
	}

	var result AntiPatternBundle
	if err := decodeJSONObject(text, &result); err != nil {
		return AntiPatternBundle{}, fmt.Errorf(
			"decode anti-pattern response: %w",
			err,
		)
	}

	result.HandsOff.Evaluated = true
	result.FeatureMarathon.Evaluated = true
	result.AIShowcase.Evaluated = true
	result.NotThinking.Evaluated = true

	return result, nil
}

func decodeJSONObject(text string, target any) error {
	text = strings.TrimSpace(text)
	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")
	text = strings.TrimSpace(text)

	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start < 0 || end < 0 || end <= start {
		return fmt.Errorf("no JSON object in response")
	}

	if err := json.Unmarshal([]byte(text[start:end+1]), target); err != nil {
		return err
	}

	return nil
}
