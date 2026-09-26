package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	anthClient "codritium/backend/internal/anthropic"
	"codritium/backend/internal/auth"
)

type ChatDeps struct {
	Pool      *pgxpool.Pool
	Anthropic *anthClient.Client
}

type chatRequest struct {
	ProblemSlug   string `json:"problem_slug"`
	SubmissionID  string `json:"submission_id"`
	Message       string `json:"message"`
	ActiveFile    string `json:"active_file"`
	FileContent   string `json:"file_content"`
}

// Chat streams a Sonnet 4.6 response over SSE.
// If submission_id is empty, a new submission row is created so the chat events
// have somewhere to land. Each user prompt and assistant chunk is logged to
// submission_events so the grader can later read the full prompt_history.
func Chat(deps ChatDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.FromContext(r.Context())
		if u == nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		var req chatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad json: "+err.Error(), http.StatusBadRequest)
			return
		}
		if req.Message == "" {
			http.Error(w, "message required", http.StatusBadRequest)
			return
		}

		ctx := r.Context()

		// Resolve problem.
		var problemID uuid.UUID
		var problemTitle, readmeMD, category, difficulty string
		err := deps.Pool.QueryRow(ctx,
			`SELECT id, title, readme_md, category, difficulty FROM problems WHERE slug=$1`,
			req.ProblemSlug,
		).Scan(&problemID, &problemTitle, &readmeMD, &category, &difficulty)
		if err != nil {
			http.Error(w, "problem not found", http.StatusNotFound)
			return
		}

		// Ensure submission exists (auto-create on first chat).
		var submissionID uuid.UUID
		if req.SubmissionID != "" {
			if id, err := uuid.Parse(req.SubmissionID); err == nil {
				submissionID = id
			}
		}
		if submissionID == uuid.Nil {
			err = deps.Pool.QueryRow(ctx, `
				INSERT INTO submissions (user_id, problem_id, status)
				VALUES ($1, $2, 'pending') RETURNING id
			`, u.ID, problemID).Scan(&submissionID)
			if err != nil {
				http.Error(w, "create submission: "+err.Error(), http.StatusInternalServerError)
				return
			}
		}

		// Log user prompt.
		_, _ = deps.Pool.Exec(ctx, `
			INSERT INTO submission_events (submission_id, event_type, role, content, selected_code, metadata)
			VALUES ($1, 'chat_prompt', 'user', $2, $3, $4::jsonb)
		`, submissionID, req.Message, req.FileContent, fmt.Sprintf(`{"active_file": %q}`, req.ActiveFile))

		// Set SSE headers.
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Submission-Id", submissionID.String())

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}

		sendEvent := func(obj map[string]any) {
			b, _ := json.Marshal(obj)
			fmt.Fprintf(w, "data: %s\n\n", b)
			flusher.Flush()
		}

		// Build system + user prompt.
		system := strings.TrimSpace(fmt.Sprintf(`You are an AI pair programming assistant inside an interview practice tool called Codritium.
The candidate is working on the problem "%s" (%s, %s difficulty).

Problem statement:
%s

Active file: %s
Current file content:
%s

Be concise. When you propose code changes, return them in a fenced code block so they can be applied directly.
Treat the candidate as a senior engineer — provide guidance, push back on unclear plans, and ask clarifying questions rather than dumping a full solution.`,
			problemTitle, category, difficulty, readmeMD, req.ActiveFile, req.FileContent))

		streamCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		defer cancel()

		stream := deps.Anthropic.Underlying().Messages.NewStreaming(streamCtx, anthropic.MessageNewParams{
			Model:     anthropic.Model(anthClient.ModelSonnet),
			MaxTokens: 1024,
			System: []anthropic.TextBlockParam{
				{Text: system},
			},
			Messages: []anthropic.MessageParam{
				anthropic.NewUserMessage(anthropic.NewTextBlock(req.Message)),
			},
		})

		var fullText strings.Builder

		for stream.Next() {
			event := stream.Current()
			switch ev := event.AsAny().(type) {
			case anthropic.ContentBlockDeltaEvent:
				if delta, ok := ev.Delta.AsAny().(anthropic.TextDelta); ok {
					fullText.WriteString(delta.Text)
					sendEvent(map[string]any{
						"type":    "chunk",
						"content": delta.Text,
					})
				}
			}
		}
		if err := stream.Err(); err != nil {
			sendEvent(map[string]any{"type": "error", "error": err.Error()})
		}

		// Send [DONE] marker.
		fmt.Fprintf(w, "data: [DONE]\n\n")
		flusher.Flush()

		// Log assistant response.
		_, _ = deps.Pool.Exec(ctx, `
			INSERT INTO submission_events (submission_id, event_type, role, content, metadata)
			VALUES ($1, 'chat_response', 'assistant', $2, $3::jsonb)
		`, submissionID, fullText.String(), fmt.Sprintf(`{"model": %q, "length": %d}`, anthClient.ModelSonnet, fullText.Len()))
	}
}
