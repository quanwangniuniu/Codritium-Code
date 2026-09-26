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

	if err := migrate.Run(ctx, database.Pool, "../migrations", "../seed"); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	if err := problems.SeedFromDir(ctx, database.Pool, "../seed/problems"); err != nil {
		log.Fatalf("seed problems: %v", err)
	}

	anthClient := anthropic.New(cfg.AnthropicAPIKey)
	e2bClient := e2b.New(cfg.E2BAPIKey)

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
	chatDeps := handlers.ChatDeps{Pool: database.Pool, Anthropic: anthClient}
	decisionWaiter := llm.NewDecisionWaiter()
	decisionDeps := handlers.DecisionDeps{Waiter: decisionWaiter}
	eventStore := events.NewStore(database.Pool)
	eventsDeps := handlers.EventsDeps{Pool: database.Pool, Events: eventStore}
	textBroadcaster := llm.NewTextBroadcaster()
	sessionsDeps := handlers.SessionsDeps{Pool: database.Pool}
	streamDeps := handlers.StreamDeps{Pool: database.Pool, Events: eventStore, Text: textBroadcaster}

	// Active chat engine: Gemini by default. CHAT_ENGINE=anthropic activates
	// the preserved Claude path and requires CLAUDE_FALLBACK_ENABLED=true
	// (enforced in config.Load).
	chatEngine := cfg.ChatEngine
	var streamClient llm.LLMStreamClient
	switch chatEngine {
	case "anthropic":
		streamClient = llm.NewClaudeStream(anthClient)
		log.Printf("chat engine: anthropic (claude_stream)")
	default:
		if geminiClient == nil {
			log.Fatalf("CHAT_ENGINE=gemini but GOOGLE_API_KEY not set")
		}
		streamClient = llm.NewGeminiStream(geminiClient)
		log.Printf("chat engine: gemini (gemini_stream)")
	}

	agentFactory := buildAgentFactory(database, streamClient, eventStore, textBroadcaster, decisionWaiter)
	agentRegistry := llm.NewAgentRegistry(agentFactory)
	chatJailbreak := llm.NewChatJailbreakClassifier()
	chatV2Deps := handlers.ChatV2Deps{
		Pool:      database.Pool,
		Registry:  agentRegistry,
		Jailbreak: chatJailbreak,
	}

	var tipsAgent *tips.Agent
	tipsFilter := tips.NewDefaultFilter()
	if geminiClient != nil {
		tipsAgent = tips.New(geminiClient, "")
		tipsFilter.Classifier = tips.NewGeminiClassifier(geminiClient, "")
		log.Printf("tips agent enabled (gemini); multiturn filter active with classifier")
	} else {
		log.Printf("tips agent disabled — GOOGLE_API_KEY not set")
	}
	tipsDeps := handlers.TipsDeps{Pool: database.Pool, Agent: tipsAgent, Filter: tipsFilter}
	subDeps := handlers.SubmissionDeps{
		Pool:         database.Pool,
		Anthropic:    anthClient,
		Gemini:       geminiClient,
		GraderEngine: cfg.GraderEngine,
	}
	commentsDeps := handlers.CommentsDeps{Pool: database.Pool}
	notesDeps := handlers.NotesDeps{Pool: database.Pool}
	replyDeps := handlers.ReplyDeps{Pool: database.Pool}

	mux := http.NewServeMux()
	authMiddleware := auth.Middleware(database.Pool, cfg.CookieSecret)

	// Three-way login wiring per login-tech impl-spec.
	oauthStateStore := auth.NewSQLOAuthStateStore(database.Pool)
	magicLinkStore := auth.NewSQLMagicLinkTokenStore(database.Pool)
	userStore := auth.NewSQLUserStore(database.Pool)
	sessionMgr := auth.NewCookieSessionManager(cfg.CookieSecret)
	googleAuth := auth.NewGoogleHandler(cfg.GoogleOAuthClientID, cfg.GoogleOAuthSecret, cfg.GoogleOAuthRedirectURL,
		oauthStateStore, userStore, sessionMgr)
	githubAuth := auth.NewGitHubHandler(cfg.GitHubOAuthClientID, cfg.GitHubOAuthSecret, cfg.GitHubOAuthRedirectURL,
		oauthStateStore, userStore, sessionMgr)
	emailAuth := auth.NewEmailHandler(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUser, cfg.SMTPPass, cfg.SMTPFrom,
		cfg.PublicURL, magicLinkStore, userStore, sessionMgr)

	// Public (no auth)
	mux.HandleFunc("GET /api/health", handlers.Health())
	mux.HandleFunc("GET /api/anthropic/ping", handlers.AnthropicPing(deps))
	mux.HandleFunc("GET /api/e2b/ping", handlers.E2BPing(deps))
	mux.HandleFunc("POST /api/auth/switch", devOnly(cfg.Env, auth.SwitchUser(database.Pool, cfg.CookieSecret)))
	mux.HandleFunc("POST /api/auth/logout", auth.Logout())

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

	// Authed routes — wrap each with authMiddleware so r.Context() carries the user.
	meHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u := auth.FromContext(r.Context())
		handlers.WriteUserJSON(w, u)
	})
	mux.Handle("GET /api/me", authMiddleware(meHandler))
	// /api/chat (v0.7 path) retired per decision_log Q2=B. Front-end uses chat_v2.
	_ = chatDeps
	mux.Handle("POST /api/submissions", authMiddleware(handlers.Submit(subDeps)))
	mux.Handle("GET /api/submissions/{id}", authMiddleware(handlers.GetSubmission(subDeps)))
	mux.Handle("DELETE /api/submissions/{id}", authMiddleware(handlers.DeleteSubmission(subDeps)))
	mux.Handle("GET /api/me/submissions", authMiddleware(handlers.ListMySubmissions(subDeps)))
	mux.Handle("POST /api/decision", authMiddleware(handlers.PostDecision(decisionDeps)))
	mux.Handle("POST /api/events", authMiddleware(handlers.PostEvent(eventsDeps)))
	mux.Handle("POST /api/sessions", authMiddleware(handlers.PostSession(sessionsDeps)))
	mux.Handle("GET /api/me/attempted", authMiddleware(handlers.ListMyAttempted(sessionsDeps)))
	mux.Handle("POST /api/chat/v2", authMiddleware(handlers.PostChatV2(chatV2Deps)))
	mux.Handle("POST /api/tips", authMiddleware(handlers.PostTips(tipsDeps)))
	mux.Handle("GET /api/tips/messages", authMiddleware(handlers.GetTipsMessages(tipsDeps)))
	mux.Handle("GET /api/sessions/{id}/stream", authMiddleware(handlers.SessionStream(streamDeps)))

	// Comments (per-problem discussion; graded gate enforced in handler).
	mux.Handle("GET /api/problems/{slug}/comments", authMiddleware(handlers.ListComments(commentsDeps)))
	mux.Handle("POST /api/problems/{slug}/comments", authMiddleware(handlers.CreateComment(commentsDeps)))
	mux.Handle("POST /api/comments/{id}/vote", authMiddleware(handlers.VoteComment(commentsDeps)))
	mux.Handle("DELETE /api/comments/{id}", authMiddleware(handlers.DeleteComment(commentsDeps)))

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
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
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

// devOnly returns 404 unless ENV=dev. Use to gate handlers that exist for
// local iteration (mock login, fixture resetters) and must never be callable
// in staging or prod, regardless of CORS or auth posture.
func devOnly(env string, h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if env != "dev" {
			http.NotFound(w, r)
			return
		}
		h(w, r)
	}
}

// graderSandboxRunner adapts grader.RunPytest to llm.SandboxRunner so the
// candidate-facing RunTests tool reuses the same E2B path the post-submission
// grader uses. Hidden test content is left empty — the candidate runs tests
// they wrote themselves, and those tests are part of `files`. The sandbox
// wrapper drops `files` into the workspace then invokes pytest against
// `testFile` directly.
type graderSandboxRunner struct{}

func (graderSandboxRunner) RunPytest(ctx context.Context, files map[string]string, testFile string) (llm.SandboxResult, error) {
	out, err := grader.RunPytest(ctx, grader.SandboxInput{
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
func buildAgentFactory(pool *db.DB, stream llm.LLMStreamClient, store *events.Store, text *llm.TextBroadcaster, waiter *llm.DecisionWaiter) llm.AgentFactory {
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
			Sandbox:      graderSandboxRunner{},
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
	return "You are the candidate-agent inside Codritium. The user is a software engineering candidate" +
		" working on the following problem. Follow their lead — do not auto-execute changes. Every FileEdit" +
		" you propose must be reviewed by the candidate before it lands.\n\n" +
		"Problem README:\n" + readme + "\n\n" +
		"Workspace files (visible):\n" + fileList + "\n" +
		"There is no pre-supplied test file. To verify your changes you (or the candidate) must create a pytest" +
		" file (e.g. `test_my.py`) via FileEdit, then call RunTests with that path. The hidden test " + hiddenTest +
		" is invisible to you and protected from access — do not try to read or modify it."
}
