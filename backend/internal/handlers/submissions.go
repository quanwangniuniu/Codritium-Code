package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"codritium/backend/internal/problems"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/genai"

	anthClient "codritium/backend/internal/anthropic"
	"codritium/backend/internal/auth"
	"codritium/backend/internal/grader"
	"codritium/backend/internal/llm"
)

type SubmissionDeps struct {
	Pool         *pgxpool.Pool
	Anthropic    *anthClient.Client
	Gemini       *genai.Client
	Ollama       *grader.OllamaClient
	GraderEngine string
	Sandbox      grader.Sandbox

	// Agents, when set, has the session's chat agent dropped on submit so
	// its history and scratch directory don't outlive the session.
	Agents *llm.AgentRegistry
}

type submitRequest struct {
	ProblemSlug  string            `json:"problem_slug"`
	Variant      string            `json:"variant"`
	CodeFiles    map[string]string `json:"code_files"`
	SubmissionID string            `json:"submission_id"`
	SessionID    string            `json:"session_id"`
}

// submitRateLimitPerHour caps how many submissions a single user can
// trigger inside any rolling one-hour window. Each submission burns
// E2B sandbox time + a 5-dim grader run, so we set the ceiling well
// below what an organic user can produce while still leaving headroom
// for retry storms.
const submitRateLimitPerHour = 5

// Submit creates (or reuses) a submission, runs the sandbox + grader, returns
// {id} immediately and grades asynchronously.
func Submit(deps SubmissionDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.FromContext(r.Context())
		if u == nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		var req submitRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad json: "+err.Error(), http.StatusBadRequest)
			return
		}
		if req.Variant == "" {
			req.Variant = "as-is"
		}

		ctx := r.Context()

		// Per-user rate limit. A reused pending submission (req.SubmissionID
		// set) isn't a new burn — count only fresh inserts.
		if req.SubmissionID == "" {
			var recent int
			if err := deps.Pool.QueryRow(ctx, `
				SELECT COUNT(*) FROM submissions
				WHERE user_id = $1 AND submitted_at >= now() - interval '1 hour'`,
				u.ID,
			).Scan(&recent); err == nil && recent >= submitRateLimitPerHour {
				w.Header().Set("Retry-After", "3600")
				writeJSON(w, http.StatusTooManyRequests, map[string]any{
					"error":  "submission rate limit exceeded",
					"limit":  submitRateLimitPerHour,
					"window": "1 hour",
				})
				return
			}
		}

		var sessionID *uuid.UUID
		if req.SessionID != "" {
			if id, err := uuid.Parse(req.SessionID); err == nil {
				// The session's transcript feeds grading and submit tears
				// down its agent, so only the owner may attach it.
				var owner string
				if err := deps.Pool.QueryRow(ctx,
					`SELECT candidate_id FROM candidate_sessions WHERE session_id = $1`, id,
				).Scan(&owner); err != nil || owner != u.Handle {
					http.Error(w, "session not found", http.StatusNotFound)
					return
				}
				sessionID = &id
			}
		}

		// Resolve problem.
		prob, err := problems.Store{Pool: deps.Pool}.Get(ctx, req.ProblemSlug)
		if err != nil {
			http.Error(w, "problem not found", http.StatusNotFound)
			return
		}
		problemID, err := uuid.Parse(prob.ID)
		if err != nil {
			http.Error(w, "problem not found", http.StatusNotFound)
			return
		}
		problemTitle, readmeMD, difficulty := prob.Title, prob.ReadmeMD, prob.Difficulty
		hiddenTestFile, hiddenTestContent := prob.HiddenTestFile, prob.HiddenTestContent
		sampleTestFile, sampleTestContent := prob.SampleTestFilename, prob.SampleTestContent
		starter := prob.Starter.Variant(req.Variant)

		// Step 1: sample_test gate. When the problem has a sample test bundled,
		// run it first; on failure return 422 with the visible diagnostic so the
		// candidate can fix without burning a grader budget.
		if sampleTestFile != nil && sampleTestContent != nil && *sampleTestContent != "" {
			sampleCtx, sampleCancel := context.WithTimeout(ctx, 30*time.Second)
			sampleRes, sampleErr := deps.Sandbox.RunPytest(sampleCtx, grader.SandboxInput{
				StarterFiles:       starter,
				CandidateFiles:     req.CodeFiles,
				HiddenTestFilename: *sampleTestFile,
				HiddenTestContent:  *sampleTestContent,
				TimeoutSec:         15,
			})
			sampleCancel()
			if sampleErr != nil {
				writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
					"stage": "sample_test",
					"error": "sandbox: " + sampleErr.Error(),
				})
				return
			}
			if sampleRes != nil && sampleRes.PassCount < sampleRes.Total {
				writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
					"stage":    "sample_test",
					"filename": *sampleTestFile,
					"passed":   sampleRes.PassCount,
					"total":    sampleRes.Total,
					"stdout":   sampleRes.Stdout,
					"stderr":   sampleRes.Stderr,
					"advice":   "fix the sample test before submitting",
				})
				return
			}
		}

		// Reuse the pending submission (created during chat) if provided,
		// otherwise insert a new row. session_id ties the submission back to
		// the chat_v2 session so the grader can pull the conversation from
		// session_messages.
		var submissionID uuid.UUID
		if req.SubmissionID != "" {
			if id, err := uuid.Parse(req.SubmissionID); err == nil {
				submissionID = id
			}
		}

		codeJSON, _ := json.Marshal(req.CodeFiles)

		if submissionID != uuid.Nil {
			tag, err := deps.Pool.Exec(ctx, `
				UPDATE submissions SET
					code_files = $2::jsonb,
					variant = $3,
					session_id = COALESCE($4, session_id),
					status = 'grading',
					submitted_at = now()
				WHERE id = $1 AND user_id = $5`,
				submissionID, string(codeJSON), req.Variant, sessionID, u.ID)
			if err != nil {
				http.Error(w, "update submission: "+err.Error(), http.StatusInternalServerError)
				return
			}
			if tag.RowsAffected() == 0 {
				http.Error(w, "submission not found", http.StatusNotFound)
				return
			}
		} else {
			err = deps.Pool.QueryRow(ctx, `
				INSERT INTO submissions (user_id, problem_id, variant, code_files, session_id, status, submitted_at)
				VALUES ($1, $2, $3, $4::jsonb, $5, 'grading', now()) RETURNING id`,
				u.ID, problemID, req.Variant, string(codeJSON), sessionID,
			).Scan(&submissionID)
			if err != nil {
				http.Error(w, "create submission: "+err.Error(), http.StatusInternalServerError)
				return
			}
		}

		// Close the session: it must not be offered for resume (its agent
		// and history are about to go), and the in-memory agent is no
		// longer needed — the transcript is already in session_messages.
		if sessionID != nil {
			if _, err := deps.Pool.Exec(ctx, `
				UPDATE candidate_sessions SET submitted_at = now()
				WHERE session_id = $1 AND submitted_at IS NULL`, *sessionID); err != nil {
				log.Printf("[submit %s] mark session %s submitted: %v", submissionID, *sessionID, err)
			}
			if deps.Agents != nil {
				deps.Agents.Drop(*sessionID)
			}
		}

		// Spin grader off in background.
		go runGradingPipeline(deps, submissionID, problemTitle, difficulty, readmeMD,
			starter, req.CodeFiles, hiddenTestFile, hiddenTestContent)

		writeJSON(w, http.StatusAccepted, map[string]any{
			"id":     submissionID,
			"status": "grading",
		})
	}
}

func runGradingPipeline(deps SubmissionDeps, submissionID uuid.UUID,
	problemTitle, difficulty, readmeMD string,
	starter, candidate map[string]string, hiddenTestFile, hiddenTestContent string,
) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// 1) Sandbox (pytest in E2B), with exponential-backoff retry on transient
	// failures. E2B occasionally returns 5xx during cold start; retry buys us
	// the second-attempt success rate without making real bugs slower to fail.
	var sb *grader.SandboxOutput
	err := retryWithBackoff(func() error {
		var e error
		sb, e = deps.Sandbox.RunPytest(ctx, grader.SandboxInput{
			StarterFiles:       starter,
			CandidateFiles:     candidate,
			HiddenTestFilename: hiddenTestFile,
			HiddenTestContent:  hiddenTestContent,
			TimeoutSec:         90,
		})
		return e
	}, 3)
	if err != nil {
		log.Printf("[grade %s] sandbox error: %v", submissionID, err)
		markFailed(ctx, deps.Pool, submissionID, "sandbox: "+err.Error())
		return
	}

	// 2) Pull prompt history for this submission.
	history := loadPromptHistory(ctx, deps.Pool, submissionID)

	// 3) Run 5-dim grader.
	graderInput := grader.GraderInput{
		ProblemTitle:      problemTitle,
		ProblemDifficulty: difficulty,
		ProblemReadme:     readmeMD,
		StarterFiles:      starter,
		CandidateFiles:    candidate,
		PromptHistory:     history,
		TestResults:       sb,
	}
	var gres *grader.GraderResult
	switch deps.GraderEngine {
	case "ollama":
		if deps.Ollama == nil {
			err = fmt.Errorf("ollama engine selected but client not initialized")
		} else {
			gres, err = grader.RunOllama(ctx, deps.Ollama, graderInput)
		}
	case "gemini":
		if deps.Gemini == nil {
			err = fmt.Errorf("gemini engine selected but client not initialized")
		} else {
			gres, err = grader.RunGemini(ctx, deps.Gemini, graderInput)
		}
	case "anthropic":
		if deps.Anthropic == nil {
			err = fmt.Errorf("anthropic engine selected but client not initialized")
		} else {
			gres, err = grader.Run(ctx, deps.Anthropic, graderInput)
		}
	default:
		err = fmt.Errorf("unsupported grader engine %q", deps.GraderEngine)
	}
	if err != nil {
		log.Printf("[grade %s] rubric error: %v", submissionID, err)
		markFailed(ctx, deps.Pool, submissionID, "grader: "+err.Error())
		return
	}

	testJSON, _ := json.Marshal(sb)
	scoresJSON, _ := json.Marshal(gres)

	_, err = deps.Pool.Exec(ctx, `
		UPDATE submissions SET
			test_results = $2::jsonb,
			scores = $3::jsonb,
			final_score = $4,
			status = 'graded',
			graded_at = now()
		WHERE id = $1`,
		submissionID, string(testJSON), string(scoresJSON), gres.FinalScore,
	)
	if err != nil {
		log.Printf("[grade %s] persist error: %v", submissionID, err)
	}
	log.Printf("[grade %s] done, final=%.1f tests=%d/%d dims=%v",
		submissionID, gres.FinalScore, sb.PassCount, sb.Total, gres.EvaluatedDims)
}

// loadPromptHistory pulls the chat_v2 transcript that produced this
// submission. Submissions created before chat_v2 (or via a pure non-chat
// flow) carry a null session_id and return an empty history.
//
// session_messages.content is a JSONB array of Anthropic content blocks
// (text / tool_use / tool_result); we flatten to plain text and stub
// tool_use blocks as "<tool:Name>" so the grader sees the model's intent
// without the full payload bloating the prompt.
func loadPromptHistory(ctx context.Context, pool *pgxpool.Pool, submissionID uuid.UUID) []grader.PromptHistoryItem {
	var sessionID *uuid.UUID
	if err := pool.QueryRow(ctx,
		`SELECT session_id FROM submissions WHERE id = $1`,
		submissionID,
	).Scan(&sessionID); err != nil || sessionID == nil {
		return nil
	}

	rows, err := pool.Query(ctx, `
		SELECT role, content::text
		FROM session_messages
		WHERE session_id = $1
		ORDER BY seq ASC`, *sessionID)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var hist []grader.PromptHistoryItem
	for rows.Next() {
		var role string
		var content []byte
		if err := rows.Scan(&role, &content); err != nil {
			continue
		}
		text := flattenAnthropicBlocks(content)
		if text == "" {
			continue
		}
		hist = append(hist, grader.PromptHistoryItem{Role: role, Content: text})
	}
	return hist
}

// flattenAnthropicBlocks extracts plain text from the JSONB content array
// stored in session_messages.content. text blocks contribute their text;
// tool_use blocks become a "<tool:Name>" marker; tool_result blocks are
// skipped because the result is already implicit in the next user / model
// turn and they would otherwise dominate the grader prompt budget.
func flattenAnthropicBlocks(raw []byte) string {
	var blocks []map[string]any
	if err := json.Unmarshal(raw, &blocks); err != nil {
		var s string
		if err2 := json.Unmarshal(raw, &s); err2 == nil {
			return s
		}
		return ""
	}
	var parts []string
	for _, b := range blocks {
		t, _ := b["type"].(string)
		switch t {
		case "text":
			if s, ok := b["text"].(string); ok && s != "" {
				parts = append(parts, s)
			}
		case "tool_use":
			if name, ok := b["name"].(string); ok && name != "" {
				parts = append(parts, "<tool:"+name+">")
			}
		}
	}
	return strings.Join(parts, "\n")
}

func markFailed(ctx context.Context, pool *pgxpool.Pool, id uuid.UUID, msg string) {
	_, _ = pool.Exec(ctx, `
		UPDATE submissions SET status='failed', scores=$2::jsonb, graded_at=now()
		WHERE id=$1`, id, fmt.Sprintf(`{"error":%q}`, msg))
}

// retryWithBackoff runs fn up to maxAttempts times with exponential delay
// (1s, 2s, 4s, ...). Returns the last error if every attempt failed.
func retryWithBackoff(fn func() error, maxAttempts int) error {
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	backoff := 1 * time.Second
	var lastErr error
	for i := 0; i < maxAttempts; i++ {
		if err := fn(); err == nil {
			return nil
		} else {
			lastErr = err
		}
		if i == maxAttempts-1 {
			break
		}
		time.Sleep(backoff)
		backoff *= 2
	}
	return lastErr
}

// ListMySubmissions returns the current user's submissions, newest first,
// trimmed to fields the listing UI needs (no code_files / no test_results).
func ListMySubmissions(deps SubmissionDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.FromContext(r.Context())
		if u == nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		rows, err := deps.Pool.Query(r.Context(), `
			SELECT s.id, p.slug, p.title, p.category, p.difficulty,
			       s.status, s.variant, s.final_score, s.submitted_at, s.graded_at
			FROM submissions s
			JOIN problems p ON p.id = s.problem_id
			WHERE s.user_id = $1
			ORDER BY s.submitted_at DESC
			LIMIT 200`, u.ID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		defer rows.Close()

		type item struct {
			ID           string     `json:"id"`
			ProblemSlug  string     `json:"problem_slug"`
			ProblemTitle string     `json:"problem_title"`
			Category     string     `json:"category"`
			Difficulty   string     `json:"difficulty"`
			Status       string     `json:"status"`
			Variant      string     `json:"variant"`
			FinalScore   *float64   `json:"final_score"`
			SubmittedAt  time.Time  `json:"submitted_at"`
			GradedAt     *time.Time `json:"graded_at"`
		}
		out := []item{}
		for rows.Next() {
			var it item
			if err := rows.Scan(&it.ID, &it.ProblemSlug, &it.ProblemTitle, &it.Category, &it.Difficulty,
				&it.Status, &it.Variant, &it.FinalScore, &it.SubmittedAt, &it.GradedAt); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
				return
			}
			out = append(out, it)
		}
		writeJSON(w, http.StatusOK, out)
	}
}

// DeleteSubmission removes one of the caller's own submissions. Ownership is
// enforced server-side: a user can never delete another user's row even by
// guessing a uuid.
func DeleteSubmission(deps SubmissionDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.FromContext(r.Context())
		if u == nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		idStr := r.PathValue("id")
		id, err := uuid.Parse(idStr)
		if err != nil {
			http.Error(w, "bad id", http.StatusBadRequest)
			return
		}
		tag, err := deps.Pool.Exec(r.Context(),
			`DELETE FROM submissions WHERE id = $1 AND user_id = $2`, id, u.ID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		if tag.RowsAffected() == 0 {
			// Either the row doesn't exist or belongs to someone else — same response
			// either way to avoid leaking existence.
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// GetSubmission returns the submission row with scores + tests. Emits a
// weak ETag of (status, graded_at) so frontend polling can short-circuit on
// 304 while the row is still grading. Body is only re-serialized when the
// status or graded_at changes. Ownership is enforced server-side: a row that
// exists but belongs to another user returns the same 404 as a missing row
// so existence cannot be probed.
func GetSubmission(deps SubmissionDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.FromContext(r.Context())
		if u == nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		idStr := r.PathValue("id")
		id, err := uuid.Parse(idStr)
		if err != nil {
			http.Error(w, "bad id", http.StatusBadRequest)
			return
		}
		var (
			problemSlug, status, variant   string
			codeFiles, testResults, scores []byte
			finalScore                     *float64
			submittedAt, gradedAt          *time.Time
			userID                         uuid.UUID
		)
		err = deps.Pool.QueryRow(r.Context(), `
			SELECT s.user_id, p.slug, s.status, s.variant, s.code_files::text,
			       COALESCE(s.test_results::text,'null'), COALESCE(s.scores::text,'null'),
			       s.final_score, s.submitted_at, s.graded_at
			FROM submissions s
			JOIN problems p ON p.id = s.problem_id
			WHERE s.id = $1 AND s.user_id = $2`, id, u.ID,
		).Scan(&userID, &problemSlug, &status, &variant, &codeFiles, &testResults, &scores, &finalScore, &submittedAt, &gradedAt)
		if err != nil {
			http.Error(w, "submission not found", http.StatusNotFound)
			return
		}

		var gradedUnix int64
		if gradedAt != nil {
			gradedUnix = gradedAt.UnixNano()
		}
		etag := fmt.Sprintf(`W/"%s-%d"`, status, gradedUnix)
		if r.Header.Get("If-None-Match") == etag {
			w.Header().Set("ETag", etag)
			w.WriteHeader(http.StatusNotModified)
			return
		}

		var code map[string]string
		_ = json.Unmarshal(codeFiles, &code)
		var tests interface{}
		_ = json.Unmarshal(testResults, &tests)
		var sc interface{}
		_ = json.Unmarshal(scores, &sc)

		w.Header().Set("ETag", etag)
		writeJSON(w, http.StatusOK, map[string]any{
			"id":           id,
			"user_id":      userID,
			"problem_slug": problemSlug,
			"status":       status,
			"variant":      variant,
			"code_files":   code,
			"test_results": tests,
			"scores":       sc,
			"final_score":  finalScore,
			"submitted_at": submittedAt,
			"graded_at":    gradedAt,
		})
	}
}
