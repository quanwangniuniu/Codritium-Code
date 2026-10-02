package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"google.golang.org/genai"

	"codritium/backend/internal/anthropic"
	"codritium/backend/internal/auth"
	"codritium/backend/internal/config"
	"codritium/backend/internal/db"
	"codritium/backend/internal/e2b"
	"codritium/backend/internal/events"
	"codritium/backend/internal/grader"
	"codritium/backend/internal/handlers"
	"codritium/backend/internal/llm"
	"codritium/backend/internal/migrate"
	"codritium/backend/internal/problems"
	"codritium/backend/internal/tips"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	log.Printf("startup: env=%s chat_engine=%s grader_engine=%s claude_fallback_enabled=%v",
		cfg.Env, cfg.ChatEngine, cfg.GraderEngine, cfg.ClaudeFallbackEnabled)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	database, err := db.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}
	defer database.Close()

	if err := migrate.Run(ctx, database.Pool, cfg.Paths.Migrations, cfg.Paths.Seed); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	if err := problems.SeedFromDir(ctx, database.Pool, cfg.Paths.ProblemsSeed); err != nil {
		log.Fatalf("seed problems: %v", err)
	}

	anthClient := anthropic.New(cfg.AnthropicAPIKey)
	e2bClient := e2b.New(cfg.E2BAPIKey)
	ollamaClient := grader.NewOllamaClient(
		cfg.OllamaBaseURL,
		cfg.OllamaModel,
		time.Duration(cfg.OllamaTimeoutSec)*time.Second,
	)

	// genai.Client has no Close() in v1.57.0 — its HTTP transport is reused
	// for the process lifetime and cleaned up by GC, so no explicit shutdown.
	var geminiClient *genai.Client
	if cfg.GoogleAPIKey != "" {
		geminiClient, err = genai.NewClient(ctx, &genai.ClientConfig{
			APIKey:  cfg.GoogleAPIKey,
			Backend: genai.BackendGeminiAPI,
		})
		if err != nil {
			log.Fatalf("gemini client: %v", err)
		}
		log.Printf("gemini client initialized (engine=%s)", cfg.GraderEngine)
	}

	deps := handlers.HealthDeps{
		Anthropic: anthClient,
		E2B:       e2bClient,
	}
	probDeps := handlers.ProblemDeps{Pool: database.Pool}
	decisionWaiter := llm.NewDecisionWaiter()
	decisionDeps := handlers.DecisionDeps{Waiter: decisionWaiter, Sessions: handlers.PGSessionOwner{Pool: database.Pool}}
	eventStore := events.NewStore(database.Pool)
	eventsDeps := handlers.EventsDeps{Pool: database.Pool, Events: eventStore}
	textBroadcaster := llm.NewTextBroadcaster()
	sessionsDeps := handlers.SessionsDeps{Pool: database.Pool}
	streamDeps := handlers.StreamDeps{Pool: database.Pool, Events: eventStore, Text: textBroadcaster}

	var streamClient llm.LLMStreamClient

	switch cfg.ChatEngine {
	case "disabled":
		log.Printf("IDE assistant disabled")
	case "ollama":
		streamClient = llm.NewOllamaStream(
			cfg.OllamaBaseURL,
			cfg.OllamaModel,
			time.Duration(cfg.OllamaTimeoutSec)*time.Second,
		)
		log.Printf("IDE assistant: ollama model=%s", cfg.OllamaModel)
	}
	sandbox := grader.Sandbox{
		Python: cfg.Paths.SandboxPython,
		Script: cfg.Paths.SandboxScript,
	}
	agentFactory := buildAgentFactory(
		database,
		streamClient,
		eventStore,
		textBroadcaster,
		decisionWaiter,
		sandbox,
	)
	agentRegistry := llm.NewAgentRegistry(agentFactory)
	chatJailbreak := llm.NewChatJailbreakClassifier()
	chatV2Deps := handlers.ChatV2Deps{
		Pool:      database.Pool,
		Registry:  agentRegistry,
		Jailbreak: chatJailbreak,
	}

	var tipsAgent *tips.Agent
	tipsFilter := tips.NewDefaultFilter()
	if streamClient != nil {
		tipsAgent = tips.New(streamClient, cfg.OllamaModel)
		tipsFilter.Classifier = tips.NewOllamaClassifier(streamClient)
		log.Printf(
			"tutor enabled: ollama model=%s",
			cfg.OllamaModel,
		)
	} else {
		log.Printf("tutor disabled with IDE assistant")
	}
	tipsDeps := handlers.TipsDeps{Pool: database.Pool, Agent: tipsAgent, Filter: tipsFilter}
	subDeps := handlers.SubmissionDeps{
		Pool:         database.Pool,
		Anthropic:    anthClient,
		Gemini:       geminiClient,
		Ollama:       ollamaClient,
		GraderEngine: cfg.GraderEngine,
		Sandbox:      sandbox,
		Agents:       agentRegistry,
	}
	commentsDeps := handlers.CommentsDeps{Pool: database.Pool}
	forumDeps := handlers.ForumDeps{Pool: database.Pool}
	notesDeps := handlers.NotesDeps{Pool: database.Pool}
	replyDeps := handlers.ReplyDeps{Pool: database.Pool}

	mux := http.NewServeMux()
	authMiddleware := auth.Middleware(database.Pool, cfg.CookieSecret)

	// Three-way login wiring per login-tech impl-spec.
	oauthStateStore := auth.NewSQLOAuthStateStore(database.Pool)
	magicLinkStore := auth.NewSQLMagicLinkTokenStore(database.Pool)
	userStore := auth.NewSQLUserStore(database.Pool)
	cookieOpts := auth.CookieOptions{Domain: cfg.CookieDomain, Secure: cfg.CookieSecure}
	sessionMgr := auth.NewCookieSessionManager(cfg.CookieSecret, cookieOpts)
	googleAuth := auth.NewGoogleHandler(cfg.GoogleOAuthClientID, cfg.GoogleOAuthSecret, cfg.GoogleOAuthRedirectURL,
		oauthStateStore, userStore, sessionMgr)
	githubAuth := auth.NewGitHubHandler(cfg.GitHubOAuthClientID, cfg.GitHubOAuthSecret, cfg.GitHubOAuthRedirectURL,
		oauthStateStore, userStore, sessionMgr)
	emailAuth := auth.NewEmailHandler(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUser, cfg.SMTPPass, cfg.SMTPFrom,
		cfg.PublicURL, magicLinkStore, userStore, sessionMgr)
	passwordAuth := auth.NewPasswordAuthHandler(database.Pool, cfg.CookieSecret, cookieOpts)

	// Public (no auth)
	mux.HandleFunc("GET /api/health", handlers.Health())
	mux.HandleFunc("GET /api/anthropic/ping", handlers.AnthropicPing(deps))
	mux.HandleFunc("GET /api/e2b/ping", handlers.E2BPing(deps))
	mux.HandleFunc("POST /api/auth/register", passwordAuth.Register)
	mux.HandleFunc("POST /api/auth/login", passwordAuth.Login)
	mux.HandleFunc("POST /api/auth/logout", auth.Logout(cookieOpts))

	// Three-way login (Google + GitHub + email magic link).
	mux.HandleFunc("POST /api/auth/google/start", googleAuth.Start)
	mux.HandleFunc("GET /api/auth/google/callback", googleAuth.Callback)
	mux.HandleFunc("POST /api/auth/github/start", githubAuth.Start)
	mux.HandleFunc("GET /api/auth/github/callback", githubAuth.Callback)
	mux.HandleFunc("POST /api/auth/email/request", emailAuth.Request)
	mux.HandleFunc("GET /api/auth/email/verify", emailAuth.Verify)
	mux.Handle("GET /api/problems", authMiddleware(http.HandlerFunc(handlers.ListProblems(probDeps))))
	mux.Handle("GET /api/problems/{slug}", authMiddleware(http.HandlerFunc(handlers.GetProblem(probDeps))))
	mux.Handle("GET /api/challenges/{slug}/official-reply", authMiddleware(handlers.GetOfficialReply(replyDeps)))
	mux.Handle("GET /api/me/replays/{slug}", authMiddleware(handlers.GetMyReplay(replyDeps)))
	mux.Handle(
		"GET /api/sessions/{id}/reply",
		authMiddleware(handlers.GetSessionReplay(replyDeps)),
	)

	// Authed routes — wrap each with authMiddleware so r.Context() carries the user.
	meHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u := auth.FromContext(r.Context())
		handlers.WriteUserJSON(w, u)
	})
	mux.Handle("GET /api/me", authMiddleware(meHandler))
	profileDeps := handlers.ProfileDeps{Pool: database.Pool}
	mux.Handle("PATCH /api/me", authMiddleware(handlers.UpdateMe(profileDeps)))
	mux.Handle("GET /api/me/profile", authMiddleware(handlers.GetMyProfile(profileDeps)))
	mux.Handle("POST /api/submissions", authMiddleware(handlers.Submit(subDeps)))
	mux.Handle("GET /api/submissions/{id}", authMiddleware(handlers.GetSubmission(subDeps)))
	mux.Handle("DELETE /api/submissions/{id}", authMiddleware(handlers.DeleteSubmission(subDeps)))
	mux.Handle("GET /api/me/submissions", authMiddleware(handlers.ListMySubmissions(subDeps)))
	mux.Handle("POST /api/decision", authMiddleware(handlers.PostDecision(decisionDeps)))
	mux.Handle("POST /api/events", authMiddleware(handlers.PostEvent(eventsDeps)))
	mux.Handle("POST /api/sessions", authMiddleware(handlers.PostSession(sessionsDeps)))
	mux.Handle("GET /api/me/attempted", authMiddleware(handlers.ListMyAttempted(sessionsDeps)))
	if cfg.ChatEngine == "disabled" {
		mux.Handle(
			"POST /api/chat/v2",
			authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "chat engine disabled", http.StatusServiceUnavailable)
			})),
		)
	} else {
		mux.Handle(
			"POST /api/chat/v2",
			authMiddleware(handlers.PostChatV2(chatV2Deps)),
		)
	}
	mux.Handle("POST /api/tips", authMiddleware(handlers.PostTips(tipsDeps)))
	mux.Handle("GET /api/tips/messages", authMiddleware(handlers.GetTipsMessages(tipsDeps)))
	mux.Handle("GET /api/sessions/{id}/stream", authMiddleware(handlers.SessionStream(streamDeps)))

	// Comments (per-problem discussion; graded gate enforced in handler).
	mux.Handle("GET /api/problems/{slug}/comments", authMiddleware(handlers.ListComments(commentsDeps)))
	mux.Handle("POST /api/problems/{slug}/comments", authMiddleware(handlers.CreateComment(commentsDeps)))
	mux.Handle("POST /api/comments/{id}/vote", authMiddleware(handlers.VoteComment(commentsDeps)))
	mux.Handle("DELETE /api/comments/{id}", authMiddleware(handlers.DeleteComment(commentsDeps)))

	// Community forum. Reads are public (the middleware leaves signed-out
	// users nil); writes require a session, enforced in the handlers.
	mux.Handle("GET /api/forum/posts", authMiddleware(handlers.ListForumPosts(forumDeps)))
	mux.Handle("GET /api/forum/pinned", authMiddleware(handlers.ListPinnedForumPosts(forumDeps)))
	mux.Handle("GET /api/forum/trending", authMiddleware(handlers.ListTrendingForumPosts(forumDeps)))
	mux.Handle("POST /api/forum/posts", authMiddleware(handlers.CreateForumPost(forumDeps)))
	mux.Handle("GET /api/forum/posts/{id}", authMiddleware(handlers.GetForumPost(forumDeps)))
	mux.Handle("PUT /api/forum/posts/{id}", authMiddleware(handlers.UpdateForumPost(forumDeps)))
	mux.Handle("DELETE /api/forum/posts/{id}", authMiddleware(handlers.DeleteForumPost(forumDeps)))
	mux.Handle("POST /api/forum/posts/{id}/vote", authMiddleware(handlers.VoteForumPost(forumDeps)))
	mux.Handle("POST /api/forum/posts/{id}/pin", authMiddleware(handlers.PinForumPost(forumDeps)))
	mux.Handle("GET /api/forum/posts/{id}/comments", authMiddleware(handlers.ListForumComments(forumDeps)))
	mux.Handle("POST /api/forum/posts/{id}/comments", authMiddleware(handlers.CreateForumComment(forumDeps)))
	mux.Handle("POST /api/forum/comments/{id}/vote", authMiddleware(handlers.VoteForumComment(forumDeps)))
	mux.Handle("DELETE /api/forum/comments/{id}", authMiddleware(handlers.DeleteForumComment(forumDeps)))

	// Candidate notes (private to the session owner; share publishes a comment).
	mux.Handle("GET /api/sessions/{id}/notes", authMiddleware(handlers.ListNotes(notesDeps)))
	mux.Handle("POST /api/sessions/{id}/notes", authMiddleware(handlers.CreateNote(notesDeps)))
	mux.Handle("PUT /api/notes/{id}", authMiddleware(handlers.UpdateNote(notesDeps)))
	mux.Handle("DELETE /api/notes/{id}", authMiddleware(handlers.DeleteNote(notesDeps)))
	mux.Handle("POST /api/notes/{id}/share", authMiddleware(handlers.ShareNote(notesDeps)))

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           withCORS(withLogging(mux)),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("codritium backend listening on :%s (grader=%s)", cfg.Port, cfg.GraderEngine)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh

	log.Println("shutting down")
	shutCtx, shutCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutCancel()
	_ = srv.Shutdown(shutCtx)
}

// Allowed origins for browser fetches. Credentials cookies require an exact
// Access-Control-Allow-Origin match (not a wildcard), so mirror the request
// Origin when it appears in this set.
var allowedOrigins = map[string]bool{
	"http://localhost:3000": true,
	"http://localhost:3012": true,
}

func withCORS(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if allowedOrigins[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		h.ServeHTTP(w, r)
	})
}

func withLogging(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		h.ServeHTTP(w, r)
		log.Printf("%s %s %v", r.Method, r.URL.Path, time.Since(start))
	})
}

// graderSandboxRunner adapts grader.RunPytest to llm.SandboxRunner so the
// candidate-facing RunTests tool reuses the same E2B path the post-submission
// grader uses. Hidden test content is left empty — the candidate runs tests
// they wrote themselves, and those tests are part of `files`. The sandbox
// wrapper drops `files` into the workspace then invokes pytest against
// `testFile` directly.
type graderSandboxRunner struct{ sandbox grader.Sandbox }

func (g graderSandboxRunner) RunPytest(ctx context.Context, files map[string]string, testFile string) (llm.SandboxResult, error) {
	out, err := g.sandbox.RunPytest(ctx, grader.SandboxInput{
		CandidateFiles:     files,
		HiddenTestFilename: testFile,
		TimeoutSec:         60,
	})
	if err != nil {
		return llm.SandboxResult{}, err
	}
	if out == nil {
		return llm.SandboxResult{}, fmt.Errorf("sandbox returned nil result for %s", testFile)
	}
	names := make([]string, 0, len(out.TestResults))
	for _, t := range out.TestResults {
		names = append(names, t.Name)
	}
	return llm.SandboxResult{
		Passed:           out.PassCount,
		Failed:           out.FailCount,
		VisibleTestNames: names,
		DurationMs:       int64(out.DurationSec * 1000),
		Stdout:           out.Stdout,
	}, nil
}

// buildAgentFactory returns the closure the AgentRegistry uses to
// construct a fresh Agent the first time a candidate hits chat_v2 on
// a session. Loads the challenge's README / starter files / hidden
// test file path from the problems table; wires the engine-neutral
// stream client + DecisionWaiter + events store; locks the visible
// test path via DenyRule.
func buildAgentFactory(pool *db.DB, stream llm.LLMStreamClient, store *events.Store, text *llm.TextBroadcaster, waiter *llm.DecisionWaiter, sandbox grader.Sandbox) llm.AgentFactory {
	return func(ctx context.Context, sessionID uuid.UUID, slug string) (*llm.Agent, error) {
		var readme, hiddenTestFile string
		var starterJSON []byte
		err := pool.Pool.QueryRow(ctx, `
			SELECT readme_md, hidden_test_file, starter_files::text
			FROM problems WHERE slug = $1`, slug,
		).Scan(&readme, &hiddenTestFile, &starterJSON)
		if err != nil {
			return nil, err
		}

		// starter_files is { filename → { variant → content } } in the
		// seed JSON; flatten to the "as-is" variant for V0.
		var allFiles map[string]map[string]string
		if err := json.Unmarshal(starterJSON, &allFiles); err != nil {
			return nil, err
		}
		files := map[string]string{}
		for name, variants := range allFiles {
			if c, ok := variants["as-is"]; ok {
				files[name] = c
			}
		}

		systemPrompt := buildSystemPrompt(readme, files, hiddenTestFile)

		// Per-session scratch directory used by the Grep / Glob / RunCommand
		// tools. The hidden test file is never materialized; the runtime tools
		// also enforce Deny rules over the in-memory workspace.
		tmpFS, tmpErr := llm.NewSessionTmpFS(sessionID, []string{hiddenTestFile})
		if tmpErr != nil {
			log.Printf("session tmpfs init failed for %s: %v", sessionID, tmpErr)
		}

		return &llm.Agent{
			Stream:       stream,
			Events:       store,
			Text:         text,
			Waiter:       waiter,
			Tools:        llm.NewDefaultRegistry(),
			Workspace:    &llm.Workspace{Files: files},
			Sandbox:      graderSandboxRunner{sandbox: sandbox},
			TmpFS:        tmpFS,
			SystemPrompt: systemPrompt,
			Deny: []llm.DenyRule{
				{Tool: "FileEdit", PathPattern: hiddenTestFile, Reason: "hidden test file is protected"},
				{Tool: "FileRead", PathPattern: hiddenTestFile, Reason: "hidden test file is protected"},
				{Tool: "Grep", PathPattern: hiddenTestFile, Reason: "hidden test file is protected"},
				{Tool: "Glob", PathPattern: hiddenTestFile, Reason: "hidden test file is protected"},
			},
		}, nil
	}
}

func buildSystemPrompt(readme string, files map[string]string, hiddenTest string) string {
	var fileList string
	for name := range files {
		fileList += "  - " + name + "\n"
	}

	return "You are the coding agent inside Codritium. Work like an IDE coding assistant and complete the user's requested task using the available tools.\n\n" +
		"Workflow:\n" +
		"1. Inspect the relevant files with FileRead, Grep, or Glob before editing.\n" +
		"2. Use FileEdit to make focused changes directly in the workspace.\n" +
		"3. After an approved edit, run the relevant tests when possible.\n" +
		"4. Read every tool result before choosing the next action.\n" +
		"5. If a test fails, use its error output to make the next focused change, then rerun the test.\n" +
		"6. If a test passes, do not reread unchanged files or call more tools unnecessarily. Immediately give a short summary of the changes and test result.\n" +
		"7. Never repeat the same FileRead, Grep, or Glob call with identical arguments unless the workspace has changed since the previous call.\n\n" +
		"Do not paste large replacement files into the chat response. Use FileEdit for code changes. " +
		"Do not ask the user whether you should read files, edit code, or run tests. Call the appropriate tool directly; " +
		"the application will request approval automatically when required. " +
		"If a tool call is rejected, respect the decision, use the rejection reason as feedback, and continue appropriately.\n\n" +
		"For an existing file, FileEdit must contain path, old_text, and new_text. " +
		"old_text must exactly match one section of the current file. Keep edits focused and avoid replacing the entire file. " +
		"For a new file, omit old_text and put the complete file contents in new_text. " +
		"Never call FileEdit with missing or empty new_text.\n\n" +
		"Problem README:\n" + readme + "\n\n" +
		"Workspace files (visible):\n" + fileList + "\n" +
		"There is no pre-supplied visible test file. When useful, create a focused pytest file through FileEdit and run it with RunTests. " +
		"The hidden test file " + hiddenTest + " is protected. Never try to read, search, or modify it."
}
