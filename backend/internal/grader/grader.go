package grader

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/anthropics/anthropic-sdk-go"

	anthClient "codritium/backend/internal/anthropic"
)

// GraderInput is everything the LLM judges need.
type GraderInput struct {
	ProblemTitle      string                 `json:"problem_title"`
	ProblemDifficulty string                 `json:"problem_difficulty"`
	ProblemReadme     string                 `json:"problem_readme"`
	StarterFiles      map[string]string      `json:"starter_files"`
	CandidateFiles    map[string]string      `json:"candidate_files"`
	PromptHistory     []PromptHistoryItem    `json:"prompt_history"`
	TestResults       *SandboxOutput         `json:"test_results"`
}

type PromptHistoryItem struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// DimensionScore is the parsed output of one rubric LLM call.
type DimensionScore struct {
	Dimension string                 `json:"dimension"`
	Score     *int                   `json:"score"`
	Reasoning string                 `json:"reasoning"`
	Extras    map[string]interface{} `json:"extras,omitempty"`
	Error     string                 `json:"error,omitempty"`
}

// GraderResult is the final aggregated score for one submission.
type GraderResult struct {
	DimensionScores map[string]DimensionScore `json:"dimension_scores"`
	Weights         map[string]float64        `json:"weights"`
	FinalScore      float64                   `json:"final_score"`
	EvaluatedDims   []string                  `json:"evaluated_dims"`
}

func Run(ctx context.Context, client *anthClient.Client, input GraderInput) (*GraderResult, error) {
	weights := DifficultyWeights(input.ProblemDifficulty)

	scoresCh := make(chan DimensionScore, len(Dimensions))
	var wg sync.WaitGroup
	for _, dim := range Dimensions {
		if _, ok := weights[dim.Name]; !ok {
			continue // dimension not weighted for this difficulty, skip
		}
		wg.Add(1)
		go func(d DimensionSpec) {
			defer wg.Done()
			score := callOneRubric(ctx, client, d, input)
			scoresCh <- score
		}(dim)
	}
	wg.Wait()
	close(scoresCh)

	results := map[string]DimensionScore{}
	for s := range scoresCh {
		results[s.Dimension] = s
	}

	// OQ6 fallback: drop null-score dimensions and renormalize remaining weights.
	validWeights := map[string]float64{}
	evaluated := []string{}
	for dim, w := range weights {
		s, ok := results[dim]
		if !ok || s.Score == nil {
			continue
		}
		validWeights[dim] = w
		evaluated = append(evaluated, dim)
	}
	var totalW float64
	for _, w := range validWeights {
		totalW += w
	}
	finalScore := 0.0
	if totalW > 0 {
		for dim, w := range validWeights {
			s := results[dim]
			finalScore += float64(*s.Score) * (w / totalW)
		}
	}
	// Scale 1-5 -> 0-100.
	final100 := finalScore * 20

	return &GraderResult{
		DimensionScores: results,
		Weights:         weights,
		FinalScore:      final100,
		EvaluatedDims:   evaluated,
	}, nil
}

func callOneRubric(ctx context.Context, client *anthClient.Client, dim DimensionSpec, input GraderInput) DimensionScore {
	userPrompt := buildUserPrompt(dim, input)

	resp, err := client.Underlying().Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(anthClient.ModelSonnet),
		MaxTokens: 1024,
		System: []anthropic.TextBlockParam{
			{Text: rubricSystem},
		},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(userPrompt)),
		},
	})
	if err != nil {
		return DimensionScore{Dimension: dim.Name, Error: err.Error()}
	}

	text := ""
	for _, blk := range resp.Content {
		text += blk.Text
	}

	parsed := parseDimensionJSON(text)
	parsed.Dimension = dim.Name
	if parsed.Reasoning == "" && parsed.Error == "" {
		// Best-effort: stash raw text in reasoning so we can debug.
		parsed.Reasoning = strings.TrimSpace(text)
	}
	return parsed
}

func buildUserPrompt(dim DimensionSpec, input GraderInput) string {
	historyJSON, _ := json.Marshal(input.PromptHistory)
	candidateJSON, _ := json.Marshal(input.CandidateFiles)
	starterJSON, _ := json.Marshal(input.StarterFiles)
	testJSON := "null"
	if input.TestResults != nil {
		b, _ := json.Marshal(input.TestResults)
		testJSON = string(b)
	}

	return fmt.Sprintf(`%s

CONTEXT:
- Problem: %s
- Difficulty: %s
- Problem statement (truncated to 4000 chars):
%s

STARTER FILES:
%s

CANDIDATE'S FINAL FILES:
%s

PROMPT HISTORY (JSON list, role + content):
%s

TEST RESULTS (sandbox JSON, may be null if not yet executed):
%s

Return ONLY the JSON object specified in the schema. No markdown fences. No prose.`,
		dim.Prompt,
		input.ProblemTitle,
		input.ProblemDifficulty,
		truncate(input.ProblemReadme, 4000),
		string(starterJSON),
		string(candidateJSON),
		string(historyJSON),
		testJSON,
	)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// parseDimensionJSON extracts the {"score": ..., "reasoning": ...} object.
func parseDimensionJSON(text string) DimensionScore {
	text = strings.TrimSpace(text)
	// Strip optional markdown fences if the model produced them anyway.
	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")
	text = strings.TrimSpace(text)

	// Find first { and last } to extract JSON object.
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start < 0 || end < 0 || end <= start {
		return DimensionScore{Error: "no JSON object in response: " + truncate(text, 300)}
	}
	jsonStr := text[start : end+1]

	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &raw); err != nil {
		return DimensionScore{Error: "parse: " + err.Error() + " raw=" + truncate(jsonStr, 200)}
	}

	out := DimensionScore{Extras: map[string]interface{}{}}
	if v, ok := raw["score"]; ok {
		switch n := v.(type) {
		case float64:
			i := int(n)
			out.Score = &i
		case nil:
			out.Score = nil
		}
	}
	if v, ok := raw["reasoning"].(string); ok {
		out.Reasoning = v
	}
	for k, v := range raw {
		if k == "score" || k == "reasoning" {
			continue
		}
		out.Extras[k] = v
	}
	return out
}
