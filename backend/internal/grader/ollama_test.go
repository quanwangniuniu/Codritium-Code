package grader

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestOllamaClientChat(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("method = %s, want POST", r.Method)
			}
			if r.URL.Path != "/api/chat" {
				t.Errorf("path = %s, want /api/chat", r.URL.Path)
			}

			var request ollamaChatRequest
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Fatalf("decode request: %v", err)
			}

			if request.Model != "qwen-test" {
				t.Errorf("model = %q, want qwen-test", request.Model)
			}
			if request.Stream {
				t.Error("stream = true, want false")
			}
			if request.Think {
				t.Error("think = true, want false")
			}
			if request.Format != "json" {
				t.Errorf("format = %q, want json", request.Format)
			}
			if len(request.Messages) != 2 {
				t.Fatalf(
					"messages length = %d, want 2",
					len(request.Messages),
				)
			}

			writeOllamaTestResponse(
				t,
				w,
				`{"score":4,"reasoning":"test response"}`,
			)
		},
	))
	defer server.Close()

	client := NewOllamaClient(
		server.URL,
		"qwen-test",
		time.Second,
	)

	content, err := client.chat(
		context.Background(),
		"system prompt",
		"user prompt",
	)
	if err != nil {
		t.Fatalf("chat returned error: %v", err)
	}

	if content != `{"score":4,"reasoning":"test response"}` {
		t.Fatalf("content = %q", content)
	}
}

func TestOllamaClientChatHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			http.Error(
				w,
				"model unavailable",
				http.StatusServiceUnavailable,
			)
		},
	))
	defer server.Close()

	client := NewOllamaClient(
		server.URL,
		"qwen-test",
		time.Second,
	)

	_, err := client.chat(
		context.Background(),
		"system prompt",
		"user prompt",
	)
	if err == nil {
		t.Fatal("chat returned nil error")
	}
	if !strings.Contains(err.Error(), "status 503") {
		t.Fatalf("error = %q, want status 503", err)
	}
}

func TestRunOllamaWithoutProcessEvidence(t *testing.T) {
	requestCount := 0

	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			requestCount++
			writeOllamaTestResponse(
				t,
				w,
				`{"score":4,"reasoning":"supported by submitted artifacts"}`,
			)
		},
	))
	defer server.Close()

	client := NewOllamaClient(
		server.URL,
		"qwen-test",
		time.Second,
	)

	result, err := RunOllama(
		context.Background(),
		client,
		GraderInput{
			ProblemTitle:      "Test problem",
			ProblemDifficulty: "easy",
			ProblemReadme:     "Solve the test problem.",
			StarterFiles: map[string]string{
				"solution.py": "",
			},
			CandidateFiles: map[string]string{
				"solution.py": "print('done')",
			},
		},
	)
	if err != nil {
		t.Fatalf("RunOllama returned error: %v", err)
	}

	if requestCount != 2 {
		t.Fatalf(
			"request count = %d, want 2",
			requestCount,
		)
	}

	for _, dimension := range []string{
		"correctness",
		"verification",
	} {
		score := result.DimensionScores[dimension]
		if score.Score == nil || *score.Score != 4 {
			t.Errorf(
				"%s score = %v, want 4",
				dimension,
				score.Score,
			)
		}
	}

	for _, dimension := range []string{
		"problem_decomposition",
		"ai_collaboration",
		"communication",
	} {
		score := result.DimensionScores[dimension]
		if score.Score != nil {
			t.Errorf(
				"%s score = %v, want nil",
				dimension,
				*score.Score,
			)
		}
		if !strings.Contains(
			score.Reasoning,
			"insufficient evidence",
		) {
			t.Errorf(
				"%s reasoning = %q",
				dimension,
				score.Reasoning,
			)
		}
	}

	if result.FinalScore != 80 {
		t.Errorf(
			"final score = %v, want 80",
			result.FinalScore,
		)
	}
	if result.JudgeModel != "qwen-test" {
		t.Errorf(
			"judge model = %q, want qwen-test",
			result.JudgeModel,
		)
	}

	if result.AntiPatterns == nil {
		t.Fatal("anti-pattern result is nil")
	}

	for name, flag := range map[string]AntiPatternFlag{
		"hands_off":        result.AntiPatterns.HandsOff,
		"feature_marathon": result.AntiPatterns.FeatureMarathon,
		"ai_showcase":      result.AntiPatterns.AIShowcase,
		"not_thinking":     result.AntiPatterns.NotThinking,
	} {
		if flag.Evaluated {
			t.Errorf(
				"%s evaluated = true, want false",
				name,
			)
		}
	}
}

func TestRunOllamaWithProcessEvidence(t *testing.T) {
	requestCount := 0

	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			requestCount++

			var request ollamaChatRequest
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Fatalf("decode request: %v", err)
			}

			if len(request.Messages) != 2 {
				t.Fatalf(
					"messages length = %d, want 2",
					len(request.Messages),
				)
			}

			userPrompt := request.Messages[1].Content
			if strings.Contains(
				userPrompt,
				"four anti-patterns",
			) {
				writeOllamaTestResponse(
					t,
					w,
					`{
						"hands_off": {
							"evaluated": true,
							"triggered": true,
							"evidence": "accepted without review"
						},
						"feature_marathon": {
							"evaluated": true,
							"triggered": false,
							"evidence": ""
						},
						"ai_showcase": {
							"evaluated": true,
							"triggered": false,
							"evidence": ""
						},
						"not_thinking": {
							"evaluated": true,
							"triggered": false,
							"evidence": ""
						}
					}`,
				)
				return
			}

			writeOllamaTestResponse(
				t,
				w,
				`{"score":4,"reasoning":"process evidence available"}`,
			)
		},
	))
	defer server.Close()

	client := NewOllamaClient(
		server.URL,
		"qwen-test",
		time.Second,
	)

	result, err := RunOllama(
		context.Background(),
		client,
		GraderInput{
			ProblemTitle:      "Test problem",
			ProblemDifficulty: "hard",
			ProblemReadme:     "Solve the test problem.",
			StarterFiles: map[string]string{
				"solution.py": "",
			},
			CandidateFiles: map[string]string{
				"solution.py": "print('done')",
			},
			PromptHistory: []PromptHistoryItem{
				{
					Role:    "user",
					Content: "First inspect the failure, then implement and test the smallest fix.",
				},
				{
					Role:    "assistant",
					Content: "I will inspect the failure first.",
				},
			},
		},
	)
	if err != nil {
		t.Fatalf("RunOllama returned error: %v", err)
	}

	if requestCount != 6 {
		t.Fatalf(
			"request count = %d, want 6",
			requestCount,
		)
	}

	for _, dimension := range Dimensions {
		score := result.DimensionScores[dimension.Name]
		if score.Score == nil || *score.Score != 4 {
			t.Errorf(
				"%s score = %v, want 4",
				dimension.Name,
				score.Score,
			)
		}
	}

	if result.FinalScore != 80 {
		t.Errorf(
			"final score = %v, want 80",
			result.FinalScore,
		)
	}

	if result.AntiPatterns == nil {
		t.Fatal("anti-pattern result is nil")
	}
	if !result.AntiPatterns.HandsOff.Evaluated {
		t.Error("hands_off evaluated = false, want true")
	}
	if !result.AntiPatterns.HandsOff.Triggered {
		t.Error("hands_off triggered = false, want true")
	}
	if result.AntiPatterns.HandsOff.Evidence == "" {
		t.Error("hands_off evidence is empty")
	}
}

func writeOllamaTestResponse(
	t *testing.T,
	w http.ResponseWriter,
	content string,
) {
	t.Helper()

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(
		ollamaChatResponse{
			Message: ollamaMessage{
				Role:    "assistant",
				Content: content,
			},
			Done: true,
		},
	); err != nil {
		t.Fatalf("encode response: %v", err)
	}
}
