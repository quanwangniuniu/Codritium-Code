package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestOllamaStreamStreamsTextAndUsage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/api/chat" {
			t.Errorf("path = %s, want /api/chat", r.URL.Path)
		}

		var request ollamaStreamRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode request: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		if request.Model != "qwen3:8b" {
			t.Errorf("model = %q, want qwen3:8b", request.Model)
		}
		if !request.Stream {
			t.Error("stream = false, want true")
		}
		if request.Think {
			t.Error("think = true, want false")
		}
		if request.Options.NumPredict != 256 {
			t.Errorf(
				"num_predict = %d, want 256",
				request.Options.NumPredict,
			)
		}

		if len(request.Messages) != 2 {
			t.Errorf("messages = %d, want 2", len(request.Messages))
		} else {
			if request.Messages[0].Role != "system" {
				t.Errorf(
					"first role = %q, want system",
					request.Messages[0].Role,
				)
			}
			if request.Messages[0].Content != "system instructions" {
				t.Errorf(
					"system content = %q",
					request.Messages[0].Content,
				)
			}
			if request.Messages[1].Role != "user" {
				t.Errorf(
					"second role = %q, want user",
					request.Messages[1].Role,
				)
			}
			if request.Messages[1].Content != "hello" {
				t.Errorf(
					"user content = %q, want hello",
					request.Messages[1].Content,
				)
			}
		}

		if len(request.Tools) != 1 {
			t.Errorf("tools = %d, want 1", len(request.Tools))
		} else if request.Tools[0].Function.Name != "FileRead" {
			t.Errorf(
				"tool name = %q, want FileRead",
				request.Tools[0].Function.Name,
			)
		}

		w.Header().Set("Content-Type", "application/x-ndjson")
		fmt.Fprintln(
			w,
			`{"message":{"role":"assistant","content":"Hello "},"done":false}`,
		)
		fmt.Fprintln(
			w,
			`{"message":{"role":"assistant","content":"world."},"done":false}`,
		)
		fmt.Fprintln(
			w,
			`{"message":{"role":"assistant","content":""},"done":true,"done_reason":"stop","prompt_eval_count":42,"eval_count":7}`,
		)
	}))
	defer server.Close()

	stream := NewOllamaStream(
		server.URL,
		"qwen3:8b",
		5*time.Second,
	)

	reader, err := stream.StreamTurn(context.Background(), TurnRequest{
		SystemPrompt: "system instructions",
		Messages: []Message{
			{
				Role: "user",
				Content: []ContentBlock{
					{
						Kind: "text",
						Text: "hello",
					},
				},
			},
		},
		Tools: []ToolSpec{
			{
				Name:        "FileRead",
				Description: "Read a file.",
				InputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"path": map[string]any{
							"type": "string",
						},
					},
					"required": []string{"path"},
				},
			},
		},
		MaxOutputTokens: 256,
	})
	if err != nil {
		t.Fatalf("StreamTurn() error = %v", err)
	}
	defer reader.Close()

	chunks := collectOllamaChunks(t, reader)

	if len(chunks) != 4 {
		t.Fatalf("chunks = %#v, want 4 chunks", chunks)
	}
	if chunks[0].Kind != "text_delta" ||
		chunks[0].Text != "Hello " {
		t.Errorf("first chunk = %#v", chunks[0])
	}
	if chunks[1].Kind != "text_delta" ||
		chunks[1].Text != "world." {
		t.Errorf("second chunk = %#v", chunks[1])
	}
	if chunks[2].Kind != "usage" {
		t.Errorf("third chunk = %#v", chunks[2])
	}
	if chunks[2].InputTokens != 42 ||
		chunks[2].OutputTokens != 7 {
		t.Errorf("usage chunk = %#v", chunks[2])
	}
	if chunks[3].Kind != "message_stop" ||
		chunks[3].StopReason != "end_turn" {
		t.Errorf("last chunk = %#v", chunks[3])
	}
}

func TestOllamaStreamEncodesToolHistoryAndReturnsToolCall(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request ollamaStreamRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode request: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		if len(request.Messages) != 3 {
			t.Errorf("messages = %d, want 3", len(request.Messages))
		} else {
			assistant := request.Messages[1]
			if assistant.Role != "assistant" {
				t.Errorf(
					"assistant role = %q",
					assistant.Role,
				)
			}
			if len(assistant.ToolCalls) != 1 {
				t.Errorf(
					"assistant tool calls = %d, want 1",
					len(assistant.ToolCalls),
				)
			} else {
				call := assistant.ToolCalls[0]
				if call.Function.Name != "FileRead" {
					t.Errorf(
						"tool name = %q, want FileRead",
						call.Function.Name,
					)
				}
				if call.Function.Arguments["path"] != "main.py" {
					t.Errorf(
						"tool arguments = %#v",
						call.Function.Arguments,
					)
				}
			}

			toolResult := request.Messages[2]
			if toolResult.Role != "tool" {
				t.Errorf(
					"tool result role = %q, want tool",
					toolResult.Role,
				)
			}
			if toolResult.ToolName != "FileRead" {
				t.Errorf(
					"tool result name = %q, want FileRead",
					toolResult.ToolName,
				)
			}
			if toolResult.Content != "print('hello')" {
				t.Errorf(
					"tool result content = %q",
					toolResult.Content,
				)
			}
		}

		w.Header().Set("Content-Type", "application/x-ndjson")
		fmt.Fprintln(
			w,
			`{"message":{"role":"assistant","content":"","tool_calls":[{"type":"function","function":{"name":"FileEdit","arguments":{"path":"main.py","content":"print('fixed')"}}}]},"done":false}`,
		)
		fmt.Fprintln(
			w,
			`{"message":{"role":"assistant","content":""},"done":true,"done_reason":"stop","prompt_eval_count":80,"eval_count":12}`,
		)
	}))
	defer server.Close()

	stream := NewOllamaStream(
		server.URL,
		"qwen3:8b",
		5*time.Second,
	)

	reader, err := stream.StreamTurn(context.Background(), TurnRequest{
		Messages: []Message{
			{
				Role: "user",
				Content: []ContentBlock{
					{
						Kind: "text",
						Text: "read main.py",
					},
				},
			},
			{
				Role: "assistant",
				Content: []ContentBlock{
					{
						Kind: "tool_use",
						ToolUse: &ContentToolUse{
							ID:   "previous-call",
							Name: "FileRead",
							Input: map[string]any{
								"path": "main.py",
							},
						},
					},
				},
			},
			{
				Role: "tool",
				Content: []ContentBlock{
					{
						Kind: "tool_result",
						ToolResult: &ContentToolResult{
							ToolUseID: "previous-call",
							Output:    "print('hello')",
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("StreamTurn() error = %v", err)
	}
	defer reader.Close()

	chunks := collectOllamaChunks(t, reader)

	if len(chunks) != 5 {
		t.Fatalf("chunks = %#v, want 5 chunks", chunks)
	}
	if chunks[0].Kind != "tool_use_start" {
		t.Fatalf("first chunk = %#v", chunks[0])
	}
	if !strings.HasPrefix(chunks[0].ToolUseID, "tu-ollama-") {
		t.Fatalf(
			"unexpected tool use ID: %q",
			chunks[0].ToolUseID,
		)
	}
	if !strings.HasSuffix(chunks[0].ToolUseID, "-1") {
		t.Fatalf(
			"first tool ID should end in -1: %q",
			chunks[0].ToolUseID,
		)
	}
	if chunks[0].ToolUseName != "FileEdit" {
		t.Errorf(
			"tool use name = %q, want FileEdit",
			chunks[0].ToolUseName,
		)
	}

	if chunks[1].Kind != "tool_use_input_delta" {
		t.Fatalf("second chunk = %#v", chunks[1])
	}

	var input map[string]any
	if err := json.Unmarshal(
		[]byte(chunks[1].InputJSONDelta),
		&input,
	); err != nil {
		t.Fatalf("decode tool input: %v", err)
	}
	if input["path"] != "main.py" {
		t.Errorf("tool input = %#v", input)
	}
	if input["content"] != "print('fixed')" {
		t.Errorf("tool input = %#v", input)
	}

	if chunks[2].Kind != "tool_use_stop" {
		t.Errorf("third chunk = %#v", chunks[2])
	}
	if chunks[3].Kind != "usage" {
		t.Errorf("fourth chunk = %#v", chunks[3])
	}
	if chunks[4].Kind != "message_stop" ||
		chunks[4].StopReason != "tool_use" {
		t.Errorf("last chunk = %#v", chunks[4])
	}
}

func TestOllamaStreamToolUseIDsAreUniqueAcrossTurns(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set(
				"Content-Type",
				"application/x-ndjson",
			)
			fmt.Fprintln(
				w,
				`{"message":{"role":"assistant","content":"","tool_calls":[{"type":"function","function":{"name":"FileRead","arguments":{"path":"main.py"}}}]},"done":false}`,
			)
			fmt.Fprintln(
				w,
				`{"message":{"role":"assistant","content":""},"done":true,"done_reason":"stop"}`,
			)
		},
	))
	defer server.Close()

	stream := NewOllamaStream(
		server.URL,
		"qwen3:8b",
		5*time.Second,
	)

	readToolUseID := func() string {
		reader, err := stream.StreamTurn(
			context.Background(),
			TurnRequest{
				Messages: []Message{
					{
						Role: "user",
						Content: []ContentBlock{
							{
								Kind: "text",
								Text: "read main.py",
							},
						},
					},
				},
			},
		)
		if err != nil {
			t.Fatalf("StreamTurn() error = %v", err)
		}
		defer reader.Close()

		chunks := collectOllamaChunks(t, reader)
		for _, chunk := range chunks {
			if chunk.Kind == "tool_use_start" {
				return chunk.ToolUseID
			}
		}

		t.Fatal("tool_use_start chunk not found")
		return ""
	}

	firstID := readToolUseID()
	secondID := readToolUseID()

	if firstID == secondID {
		t.Fatalf(
			"tool IDs must be unique across turns: %q",
			firstID,
		)
	}

	if !strings.HasSuffix(firstID, "-1") {
		t.Fatalf("first ID should end in -1: %q", firstID)
	}
	if !strings.HasSuffix(secondID, "-1") {
		t.Fatalf("second ID should end in -1: %q", secondID)
	}
}

func TestOllamaStreamReturnsHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(
			w,
			`{"error":"model not found"}`,
			http.StatusNotFound,
		)
	}))
	defer server.Close()

	stream := NewOllamaStream(
		server.URL,
		"missing-model",
		5*time.Second,
	)

	_, err := stream.StreamTurn(context.Background(), TurnRequest{
		Messages: []Message{
			{
				Role: "user",
				Content: []ContentBlock{
					{
						Kind: "text",
						Text: "hello",
					},
				},
			},
		},
	})
	if err == nil {
		t.Fatal("StreamTurn() error = nil, want HTTP error")
	}
	if !strings.Contains(err.Error(), "HTTP 404") {
		t.Fatalf(
			"StreamTurn() error = %q, want HTTP 404",
			err,
		)
	}
	if !strings.Contains(err.Error(), "model not found") {
		t.Fatalf(
			"StreamTurn() error = %q, want model error",
			err,
		)
	}
}

func collectOllamaChunks(
	t *testing.T,
	reader StreamReader,
) []NormalizedChunk {
	t.Helper()

	var chunks []NormalizedChunk
	for {
		chunk, ok, err := reader.Next(context.Background())
		if err != nil {
			t.Fatalf("Next() error = %v", err)
		}
		if !ok {
			return chunks
		}
		chunks = append(chunks, chunk)
	}
}
