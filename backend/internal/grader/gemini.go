package grader

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"google.golang.org/genai"
)

const (
	geminiModelPrimary  = "gemini-2.5-flash"
	geminiModelFallback = "gemini-2.5-pro"

	// MaxOutputTokens covers thinking tokens + visible output combined.
	// 8192 leaves comfortable headroom for the longest dim (correctness +
	// edge_cases_missed array) even when dynamic thinking allocates ~4-5K
	// tokens. Flash output ceiling is 65K so 8K is well within bounds.
	geminiMaxOutputTokens = 8192
)

// RunGemini is the Gemini-only counterpart of Run. Same input/output contract.
// Differences from Anthropic path:
//   - 5 dim called serially per submission (rate-limit safety)
//   - explicit ResponseSchema per dim (avoids #2418 propertyOrdering bug)
//   - input cap + truncate strategy for long-context degradation
//   - sanitizer applied to scrub provenance metadata
func RunGemini(ctx context.Context, client *genai.Client, input GraderInput) (*GraderResult, error) {
	weights := DifficultyWeights(input.ProblemDifficulty)

	clean := SanitizeInput(input)
	clean = capInput(clean)

	results := map[string]DimensionScore{}
	for _, dim := range Dimensions {
		if _, ok := weights[dim.Name]; !ok {
			continue
		}
		results[dim.Name] = gradeOneDimGemini(ctx, client, dim, clean, geminiModelPrimary)
	}

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
	final100 := finalScore * 20

	return &GraderResult{
		DimensionScores: results,
		Weights:         weights,
		FinalScore:      final100,
		EvaluatedDims:   evaluated,
	}, nil
}

func gradeOneDimGemini(ctx context.Context, client *genai.Client, dim DimensionSpec, input GraderInput, model string) DimensionScore {
	userPrompt := buildUserPrompt(dim, input)
	schema := schemaForDim(dim.Name)

	cfg := &genai.GenerateContentConfig{
		SystemInstruction: genai.NewContentFromText(rubricSystem, genai.RoleUser),
		Temperature:       genai.Ptr[float32](0.0),
		MaxOutputTokens:   geminiMaxOutputTokens,
		ResponseMIMEType:  "application/json",
		ResponseSchema:    schema,
		ThinkingConfig: &genai.ThinkingConfig{
			ThinkingBudget: genai.Ptr[int32](-1),
		},
	}

	resp, err := callGeminiWithRetry(ctx, client, model, genai.Text(userPrompt), cfg)
	if err != nil {
		return DimensionScore{Dimension: dim.Name, Error: err.Error()}
	}

	text := resp.Text()
	// Empty response = #1039 MAX_TOKENS / safety filter / unknown. Fall back to Pro once,
	// since the bigger model has more reasoning budget and is less likely to truncate.
	if strings.TrimSpace(text) == "" && model != geminiModelFallback {
		finish := primaryFinishReason(resp)
		log.Printf("grader gemini: dim=%s empty response (finish=%s), retrying with %s", dim.Name, finish, geminiModelFallback)
		resp, err = callGeminiWithRetry(ctx, client, geminiModelFallback, genai.Text(userPrompt), cfg)
		if err != nil {
			return DimensionScore{Dimension: dim.Name, Error: "fallback failed: " + err.Error()}
		}
		text = resp.Text()
	}
	if strings.TrimSpace(text) == "" {
		return DimensionScore{Dimension: dim.Name, Error: "empty response (finish=" + primaryFinishReason(resp) + ")"}
	}

	parsed := parseDimensionJSON(text)
	parsed.Dimension = dim.Name
	if parsed.Reasoning == "" && parsed.Error == "" {
		parsed.Reasoning = strings.TrimSpace(text)
	}
	return parsed
}

func primaryFinishReason(resp *genai.GenerateContentResponse) string {
	if resp == nil || len(resp.Candidates) == 0 {
		return ""
	}
	return string(resp.Candidates[0].FinishReason)
}

// callGeminiWithRetry: 6-step exponential backoff matching Anthropic path.
func callGeminiWithRetry(ctx context.Context, client *genai.Client, model string, contents []*genai.Content, cfg *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error) {
	delays := []time.Duration{
		500 * time.Millisecond,
		1 * time.Second,
		2 * time.Second,
		4 * time.Second,
		8 * time.Second,
		16 * time.Second,
	}
	var lastErr error
	for attempt := 0; attempt <= len(delays); attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delays[attempt-1]):
			}
		}
		resp, err := client.Models.GenerateContent(ctx, model, contents, cfg)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		if !isRetriableGeminiErr(err) {
			return nil, err
		}
	}
	return nil, fmt.Errorf("gemini call exhausted retries: %w", lastErr)
}

func isRetriableGeminiErr(err error) bool {
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "429") ||
		strings.Contains(s, "rate limit") ||
		strings.Contains(s, "quota") ||
		strings.Contains(s, "503") ||
		strings.Contains(s, "unavailable") ||
		strings.Contains(s, "deadline") ||
		strings.Contains(s, "internal error") ||
		strings.Contains(s, "500")
}

// capInput trims oversized fields so total grader input stays within
// the long-context comfort zone for Gemini Flash. Strategy:
//   - problem readme: cap at 4000 chars (already done in buildUserPrompt)
//   - candidate code files: per-file cap 8000 chars
//   - prompt history: keep first item + last N items totaling ~16000 chars
func capInput(in GraderInput) GraderInput {
	out := in
	out.CandidateFiles = capFileMap(in.CandidateFiles, 8000)
	out.StarterFiles = capFileMap(in.StarterFiles, 4000)
	out.PromptHistory = capHistory(in.PromptHistory, 16000)
	return out
}

func capFileMap(files map[string]string, perFileMax int) map[string]string {
	if files == nil {
		return nil
	}
	out := make(map[string]string, len(files))
	for name, body := range files {
		if len(body) > perFileMax {
			body = body[:perFileMax] + "\n…[truncated]…"
		}
		out[name] = body
	}
	return out
}

// capHistory keeps the first turn (sets context) plus the most recent turns,
// budgeted by total character count. If a recent turn is too large to fit, it
// is truncated rather than dropped — the newest turn carries the candidate's
// current intent and should never be silently lost.
func capHistory(items []PromptHistoryItem, maxChars int) []PromptHistoryItem {
	if len(items) == 0 {
		return items
	}
	total := 0
	for _, it := range items {
		total += len(it.Content)
	}
	if total <= maxChars {
		return items
	}

	const truncMarker = "…[truncated]…"
	const gapReserve = 80 // bytes set aside for the "intermediate turns truncated" marker
	firstBudget := maxChars / 3
	first := items[0]
	if len(first.Content) > firstBudget && firstBudget > len(truncMarker) {
		first = PromptHistoryItem{
			Role:    first.Role,
			Content: first.Content[:firstBudget-len(truncMarker)] + truncMarker,
		}
	}
	if len(items) == 1 {
		return []PromptHistoryItem{first}
	}

	budget := maxChars - len(first.Content) - gapReserve
	if budget < 0 {
		budget = 0
	}
	tail := []PromptHistoryItem{}
	for i := len(items) - 1; i >= 1; i-- {
		it := items[i]
		if len(it.Content) <= budget {
			tail = append([]PromptHistoryItem{it}, tail...)
			budget -= len(it.Content)
			continue
		}
		// Doesn't fit: truncate THIS turn to whatever budget remains, then stop.
		if budget > len(truncMarker)+50 {
			trimmed := PromptHistoryItem{
				Role:    it.Role,
				Content: it.Content[:budget-len(truncMarker)] + truncMarker,
			}
			tail = append([]PromptHistoryItem{trimmed}, tail...)
		}
		break
	}

	skipped := len(items) - 1 - len(tail)
	out := []PromptHistoryItem{first}
	if skipped > 0 {
		out = append(out, PromptHistoryItem{
			Role:    "system",
			Content: fmt.Sprintf("…[%d intermediate turns truncated]…", skipped),
		})
	}
	out = append(out, tail...)
	return out
}
