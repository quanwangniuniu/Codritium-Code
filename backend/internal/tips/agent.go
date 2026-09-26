package tips

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"google.golang.org/genai"
)

const defaultModel = "gemini-2.5-flash"

// Mode identifies the session lifecycle stage the tips-agent is called from.
// Practice is the only mode where hints are served; simulator and replay
// must hard-fail upstream so we keep the enum tight here.
type Mode string

const (
	ModePractice  Mode = "practice"
	ModeSimulator Mode = "simulator"
	ModeReply     Mode = "reply"
)

// Turn is one entry in the user/tutor exchange. The tips-agent has no
// tool use, no streaming, no decision waiter — every turn is a single
// text exchange with explicit per-turn user input.
type Turn struct {
	Role string // "user" | "model"
	Text string
}

// Request is the input shape the HTTP handler hands to the agent.
// SoulPrebake is the problem-specific guidance the candidate is allowed
// to see; the seeded prebake is the only source. Conversation is the
// running multi-turn history scoped to this practice session. FileContents
// is the optional candidate-edited workspace snapshot — never the hidden
// test, only files the candidate already sees. AgentSummary is the
// rule-based digest of the candidate's chat_v2 progress; nil/empty leaves
// the prompt section out so the model behaviour matches the pre-1.0 path.
type Request struct {
	Mode         Mode
	SoulPrebake  string
	AgentSummary string
	Conversation []Turn
	FileContents map[string]string
}

// StreamChunk is one piece of a streaming tutor turn. Delta is the text
// fragment to append to the UI; Done signals the model finished; Err
// carries terminal stream errors (Done and Err are mutually exclusive).
type StreamChunk struct {
	Delta string
	Done  bool
	Err   error
}

// Response holds the tutor reply plus the resolved model name so callers
// can log or surface which engine produced the answer (Phase 1: gemini
// only; Phase 2+ may add fallbacks).
type Response struct {
	Text  string
	Model string
}

// Agent wraps a genai.Client and resolves the model name. Construct one
// per process; the SDK reuses HTTP transports for the client's lifetime.
type Agent struct {
	Client *genai.Client
	Model  string
}

// New returns an Agent. Client must be non-nil — Phase 1 has no offline
// fallback. Model defaults to gemini-2.5-flash when blank.
func New(client *genai.Client, model string) *Agent {
	if model == "" {
		model = defaultModel
	}
	return &Agent{Client: client, Model: model}
}

// Ask runs a single non-streaming tips turn and returns the tutor reply.
// Mode != ModePractice is rejected — the practice mode gate is the only
// surface that calls the agent. Empty conversation is also rejected;
// the handler is expected to require at least one user turn.
func (a *Agent) Ask(ctx context.Context, req Request) (Response, error) {
	if req.Mode != ModePractice {
		return Response{}, fmt.Errorf("tips agent: mode %q not allowed", req.Mode)
	}
	if len(req.Conversation) == 0 {
		return Response{}, errors.New("tips agent: empty conversation")
	}
	if a == nil || a.Client == nil {
		return Response{}, errors.New("tips agent: gemini client not configured")
	}

	contents := toGenAIContents(req.Conversation)
	cfg := &genai.GenerateContentConfig{
		Temperature:     genai.Ptr[float32](0.7),
		MaxOutputTokens: 4096,
		// gemini-2.5-flash enables internal thinking by default and the
		// thinking tokens count against MaxOutputTokens. For Socratic
		// tutor turns we want fast, short replies — no thinking budget.
		ThinkingConfig: &genai.ThinkingConfig{
			ThinkingBudget: genai.Ptr[int32](0),
		},
	}
	systemPrompt := BuildTipsPrompt(req.SoulPrebake, req.AgentSummary, req.FileContents)
	cfg.SystemInstruction = genai.NewContentFromText(systemPrompt, genai.RoleUser)

	resp, err := a.Client.Models.GenerateContent(ctx, a.Model, contents, cfg)
	if err != nil {
		return Response{}, fmt.Errorf("tips agent: gemini generate: %w", err)
	}

	text := extractText(resp)
	if strings.TrimSpace(text) == "" {
		return Response{}, errors.New("tips agent: empty response from gemini")
	}
	return Response{Text: text, Model: a.Model}, nil
}

// AskStream is the streaming variant of Ask. It returns a channel of
// StreamChunk values terminated by either a Done=true or an Err. The
// channel closes after the terminal chunk so callers may range over it
// without explicit length tracking. Ctx cancellation aborts the stream.
func (a *Agent) AskStream(ctx context.Context, req Request) (<-chan StreamChunk, error) {
	if req.Mode != ModePractice {
		return nil, fmt.Errorf("tips agent: mode %q not allowed", req.Mode)
	}
	if len(req.Conversation) == 0 {
		return nil, errors.New("tips agent: empty conversation")
	}
	if a == nil || a.Client == nil {
		return nil, errors.New("tips agent: gemini client not configured")
	}

	contents := toGenAIContents(req.Conversation)
	cfg := &genai.GenerateContentConfig{
		Temperature:     genai.Ptr[float32](0.7),
		MaxOutputTokens: 4096,
		// gemini-2.5-flash enables internal thinking by default and the
		// thinking tokens count against MaxOutputTokens. For Socratic
		// tutor turns we want fast, short replies — no thinking budget.
		ThinkingConfig: &genai.ThinkingConfig{
			ThinkingBudget: genai.Ptr[int32](0),
		},
	}
	systemPrompt := BuildTipsPrompt(req.SoulPrebake, req.AgentSummary, req.FileContents)
	cfg.SystemInstruction = genai.NewContentFromText(systemPrompt, genai.RoleUser)

	model := a.Model
	if model == "" {
		model = defaultModel
	}
	seq := a.Client.Models.GenerateContentStream(ctx, model, contents, cfg)

	out := make(chan StreamChunk)
	go func() {
		defer close(out)
		emitted := false
		for resp, err := range seq {
			if err != nil {
				select {
				case out <- StreamChunk{Err: fmt.Errorf("tips stream: %w", err)}:
				case <-ctx.Done():
				}
				return
			}
			text := extractText(resp)
			if text == "" {
				continue
			}
			emitted = true
			select {
			case out <- StreamChunk{Delta: text}:
			case <-ctx.Done():
				return
			}
		}
		if !emitted {
			select {
			case out <- StreamChunk{Err: errors.New("tips stream: empty response from gemini")}:
			case <-ctx.Done():
			}
			return
		}
		select {
		case out <- StreamChunk{Done: true}:
		case <-ctx.Done():
		}
	}()
	return out, nil
}

func toGenAIContents(turns []Turn) []*genai.Content {
	out := make([]*genai.Content, 0, len(turns))
	for _, t := range turns {
		var role genai.Role = genai.RoleUser
		if t.Role == "model" {
			role = genai.RoleModel
		}
		out = append(out, genai.NewContentFromText(t.Text, role))
	}
	return out
}

func extractText(resp *genai.GenerateContentResponse) string {
	if resp == nil || len(resp.Candidates) == 0 {
		return ""
	}
	var b strings.Builder
	for _, cand := range resp.Candidates {
		if cand == nil || cand.Content == nil {
			continue
		}
		for _, part := range cand.Content.Parts {
			if part == nil {
				continue
			}
			if part.Text != "" {
				b.WriteString(part.Text)
			}
		}
	}
	return b.String()
}
