// Package gemini adapts the Gemini SDK to llm.LLMStreamClient.
package gemini

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"

	"github.com/google/uuid"

	"codritium/backend/internal/llm"

	"google.golang.org/genai"
)

const defaultGeminiChatModel = "gemini-2.5-flash"

// GeminiStream adapts the google.golang.org/genai SDK streaming API to
// the engine-neutral llm.LLMStreamClient contract that chat_v2.go consumes.
//
// Gemini's streaming surface is one chunk per Part: a text Part becomes
// a text_delta; a FunctionCall Part becomes a tool_use_start +
// tool_use_input_delta (full JSON of Args) + tool_use_stop triple, so
// drainStream's incremental decoder works uniformly with Claude's
// genuinely-streamed tool_use input.
type Stream struct {
	Client *genai.Client
	Model  string // empty → "gemini-2.5-flash"
}

// New wires up the SDK client and applies engine defaults
// for the v0.8 candidate-agent. The model name follows config.GraderEngine
// semantics: gemini-2.5-flash for chat; the Pro fallback is left to the
// outer error policy, not built into the stream layer.
func New(client *genai.Client) *Stream {
	return &Stream{Client: client, Model: defaultGeminiChatModel}
}

func (g *Stream) StreamTurn(ctx context.Context, req llm.TurnRequest) (llm.StreamReader, error) {
	if g.Client == nil {
		return nil, fmt.Errorf("gemini stream: client not configured")
	}
	model := g.Model
	if model == "" {
		model = defaultGeminiChatModel
	}

	contents, err := toGenAIContents(req.Messages)
	if err != nil {
		return nil, fmt.Errorf("gemini stream: encode contents: %w", err)
	}
	cfg := &genai.GenerateContentConfig{
		Temperature:     genai.Ptr[float32](0.7),
		MaxOutputTokens: int32OrZero(req.MaxOutputTokens),
	}
	if req.SystemPrompt != "" {
		cfg.SystemInstruction = genai.NewContentFromText(req.SystemPrompt, genai.RoleUser)
	}
	if len(req.Tools) > 0 {
		cfg.Tools = []*genai.Tool{{FunctionDeclarations: toFunctionDecls(req.Tools)}}
	}

	seq := g.Client.Models.GenerateContentStream(ctx, model, contents, cfg)
	return &geminiReader{seq: seq, pull: pullStarter(seq)}, nil
}

func int32OrZero(v int) int32 {
	if v <= 0 {
		return 0
	}
	return int32(v)
}

// pullStarter wraps the iter.Seq2 from the SDK into a pull-style cursor.
// genai.GenerateContentStream returns an iter.Seq2[*GenerateContentResponse, error]
// (Go 1.23+ range-over-func). We adapt it to Next() by spawning a goroutine.
func pullStarter(seq iter.Seq2[*genai.GenerateContentResponse, error]) chan geminiPullItem {
	ch := make(chan geminiPullItem)
	go func() {
		defer close(ch)
		for resp, err := range seq {
			ch <- geminiPullItem{Resp: resp, Err: err}
			if err != nil {
				return
			}
		}
	}()
	return ch
}

type geminiPullItem struct {
	Resp *genai.GenerateContentResponse
	Err  error
}

type geminiReader struct {
	seq        iter.Seq2[*genai.GenerateContentResponse, error]
	pull       chan geminiPullItem
	queue      []llm.NormalizedChunk
	usageSent  bool
	stopSent   bool
	functionID int // counter for synthesised tool_use_ids (Gemini doesn't issue ids)
}

func (r *geminiReader) Next(ctx context.Context) (llm.NormalizedChunk, bool, error) {
	for len(r.queue) == 0 {
		select {
		case <-ctx.Done():
			return llm.NormalizedChunk{}, false, ctx.Err()
		case item, ok := <-r.pull:
			if !ok {
				// stream ended: flush any final usage / message_stop
				if !r.usageSent {
					r.usageSent = true
					r.queue = append(r.queue, llm.NormalizedChunk{Kind: "usage"})
				}
				if !r.stopSent {
					r.stopSent = true
					r.queue = append(r.queue, llm.NormalizedChunk{Kind: "message_stop", StopReason: "end_turn"})
					continue
				}
				return llm.NormalizedChunk{}, false, nil
			}
			if item.Err != nil {
				return llm.NormalizedChunk{}, false, item.Err
			}
			r.enqueueFromResponse(item.Resp)
		}
	}
	c := r.queue[0]
	r.queue = r.queue[1:]
	return c, true, nil
}

func (r *geminiReader) Close() error { return nil }

func (r *geminiReader) enqueueFromResponse(resp *genai.GenerateContentResponse) {
	if resp == nil || len(resp.Candidates) == 0 {
		return
	}
	cand := resp.Candidates[0]
	if cand.Content != nil {
		for _, p := range cand.Content.Parts {
			switch {
			case p.Text != "":
				r.queue = append(r.queue, llm.NormalizedChunk{Kind: "text_delta", Text: p.Text})
			case p.FunctionCall != nil:
				r.functionID++
				id := fmt.Sprintf("tu-gem-%d-%s", r.functionID, uuid.NewString()[:8])
				args := p.FunctionCall.Args
				inputJSON, _ := json.Marshal(args)
				r.queue = append(r.queue,
					llm.NormalizedChunk{Kind: "tool_use_start", ToolUseID: id, ToolUseName: p.FunctionCall.Name},
					llm.NormalizedChunk{Kind: "tool_use_input_delta", ToolUseID: id, InputJSONDelta: string(inputJSON)},
					llm.NormalizedChunk{Kind: "tool_use_stop", ToolUseID: id},
				)
			}
		}
	}
	if cand.FinishReason != "" && !r.stopSent {
		r.stopSent = true
		r.queue = append(r.queue, llm.NormalizedChunk{
			Kind:       "message_stop",
			StopReason: string(cand.FinishReason),
		})
	}
	if resp.UsageMetadata != nil && !r.usageSent {
		r.usageSent = true
		r.queue = append(r.queue, llm.NormalizedChunk{
			Kind:            "usage",
			InputTokens:     int(resp.UsageMetadata.PromptTokenCount),
			OutputTokens:    int(resp.UsageMetadata.CandidatesTokenCount),
			CacheReadTokens: int(resp.UsageMetadata.CachedContentTokenCount),
		})
	}
}

// ─── encoders ───────────────────────────────────────────────────────────

func toGenAIContents(msgs []llm.Message) ([]*genai.Content, error) {
	out := make([]*genai.Content, 0, len(msgs))
	for _, m := range msgs {
		role := genai.RoleUser
		if m.Role == "assistant" {
			role = genai.RoleModel
		}
		// Tool results are folded back as user-role function responses,
		// per genai SDK convention.
		if m.Role == "tool" {
			parts := make([]*genai.Part, 0, len(m.Content))
			for _, b := range m.Content {
				if b.Kind != "tool_result" || b.ToolResult == nil {
					continue
				}
				parts = append(parts, &genai.Part{
					FunctionResponse: &genai.FunctionResponse{
						Name:     b.ToolResult.ToolUseID,
						Response: map[string]any{"output": b.ToolResult.Output, "is_error": b.ToolResult.IsError},
					},
				})
			}
			if len(parts) > 0 {
				out = append(out, &genai.Content{Role: genai.RoleUser, Parts: parts})
			}
			continue
		}

		parts := make([]*genai.Part, 0, len(m.Content))
		for _, b := range m.Content {
			switch b.Kind {
			case "text":
				if b.Text != "" {
					parts = append(parts, &genai.Part{Text: b.Text})
				}
			case "tool_use":
				if b.ToolUse != nil {
					parts = append(parts, &genai.Part{
						FunctionCall: &genai.FunctionCall{
							Name: b.ToolUse.Name,
							Args: b.ToolUse.Input,
						},
					})
				}
			}
		}
		if len(parts) > 0 {
			out = append(out, &genai.Content{Role: role, Parts: parts})
		}
	}
	return out, nil
}

func toFunctionDecls(tools []llm.ToolSpec) []*genai.FunctionDeclaration {
	out := make([]*genai.FunctionDeclaration, 0, len(tools))
	for _, t := range tools {
		out = append(out, &genai.FunctionDeclaration{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  schemaFromMap(t.InputSchema),
		})
	}
	return out
}

// schemaFromMap is a best-effort converter from the engine-neutral JSON
// Schema dict to *genai.Schema. Only the subset we use in V0 tools is
// translated (object root with string properties). Unknown keys are
// dropped silently — V0 schemas are tiny.
func schemaFromMap(m map[string]any) *genai.Schema {
	if m == nil {
		return nil
	}
	s := &genai.Schema{Type: genai.TypeObject}
	if t, ok := m["type"].(string); ok {
		switch t {
		case "object":
			s.Type = genai.TypeObject
		case "string":
			s.Type = genai.TypeString
		case "integer":
			s.Type = genai.TypeInteger
		case "number":
			s.Type = genai.TypeNumber
		case "boolean":
			s.Type = genai.TypeBoolean
		}
	}
	if props, ok := m["properties"].(map[string]any); ok {
		s.Properties = map[string]*genai.Schema{}
		for k, v := range props {
			if vm, ok := v.(map[string]any); ok {
				s.Properties[k] = schemaFromMap(vm)
				if desc, ok := vm["description"].(string); ok {
					s.Properties[k].Description = desc
				}
			}
		}
	}
	if req, ok := m["required"].([]string); ok {
		s.Required = append(s.Required, req...)
	}
	return s
}
