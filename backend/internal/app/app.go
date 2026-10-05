// Package app wires the server: it builds every dependency once from
// config, registers each module's routes, and owns graceful shutdown.
// Adding a feature means adding its module to modules() — nothing else in
// this file should grow.
package app

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
	"google.golang.org/genai"

	"codritium/backend/internal/account"
	"codritium/backend/internal/agent"
	"codritium/backend/internal/anthropic"
	"codritium/backend/internal/auth"
	"codritium/backend/internal/community/comments"
	"codritium/backend/internal/community/forum"
	"codritium/backend/internal/community/notes"
	"codritium/backend/internal/config"
	"codritium/backend/internal/db"
	"codritium/backend/internal/events"
	"codritium/backend/internal/grader"
	"codritium/backend/internal/grading"
	"codritium/backend/internal/llm"
	"codritium/backend/internal/migrate"
	"codritium/backend/internal/notifications"
	"codritium/backend/internal/platform/httpx"
	"codritium/backend/internal/problems"
	"codritium/backend/internal/problems/seed"
	"codritium/backend/internal/profile"
	"codritium/backend/internal/replay"
	"codritium/backend/internal/sessions"
	"codritium/backend/internal/submissions"
	"codritium/backend/internal/tips"
)

// App is a fully wired server.
type App struct {
	cfg     *config.Config
	db      *db.DB
	grading *grading.Service
	handler http.Handler
}

// New connects to the database, migrates and seeds it, builds every
// service, and registers all routes.
func New(ctx context.Context, cfg *config.Config) (*App, error) {
	database, err := db.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("db connect: %w", err)
	}
	a := &App{cfg: cfg, db: database}
	if err := a.build(ctx); err != nil {
		database.Close()
		return nil, err
	}
	return a, nil
}

// Handler is the root HTTP handler (CORS + logging + routes).
func (a *App) Handler() http.Handler { return a.handler }

// Shutdown drains background grading (up to ctx's deadline; unfinished
// jobs are marked failed) and closes the database.
func (a *App) Shutdown(ctx context.Context) error {
	err := a.grading.Shutdown(ctx)
	a.db.Close()
	return err
}

func (a *App) build(ctx context.Context) error {
	cfg, pool := a.cfg, a.db.Pool
	if err := migrate.Run(ctx, pool, cfg.Paths.Migrations, cfg.Paths.Seed); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	if err := seed.FromDir(ctx, pool, cfg.Paths.ProblemsSeed); err != nil {
		return fmt.Errorf("seed problems: %w", err)
	}

	// Provider clients. genai.Client has no Close; it lives for the process.
	var gemini *genai.Client
	if cfg.GoogleAPIKey != "" {
		var err error
		gemini, err = genai.NewClient(ctx, &genai.ClientConfig{APIKey: cfg.GoogleAPIKey, Backend: genai.BackendGeminiAPI})
		if err != nil {
			return fmt.Errorf("gemini client: %w", err)
		}
	}
	var claude *anthropic.Client
	if cfg.AnthropicAPIKey != "" {
		claude = anthropic.New(cfg.AnthropicAPIKey)
	}

	// Shared stores and services.
	problemStore := problems.Store{Pool: pool}
	sessionStore := sessions.Store{Pool: pool}
	sandbox := grader.Sandbox{Python: cfg.Paths.SandboxPython, Script: cfg.Paths.SandboxScript}
	eventStore := events.NewStore(pool)
	text := llm.NewTextBroadcaster()
	waiter := llm.NewDecisionWaiter()

	engine, err := grader.NewEngine(cfg.GraderEngine, grader.Clients{
		Anthropic: claude,
		Gemini:    gemini,
		Ollama:    grader.NewOllamaClient(cfg.OllamaBaseURL, cfg.OllamaModel, time.Duration(cfg.OllamaTimeoutSec)*time.Second),
	})
	if err != nil {
		return err
	}
	a.grading = &grading.Service{Pool: pool, Sandbox: sandbox, Engine: engine}

	// The coding agent uses the configured local Ollama model. When chat is
	// disabled, the agent routes remain available but return 503.
	var agents *llm.AgentRegistry
	if cfg.ChatEngine == "ollama" {
		stream := llm.NewOllamaStream(
			cfg.OllamaBaseURL,
			cfg.OllamaModel,
			time.Duration(cfg.OllamaTimeoutSec)*time.Second,
		)

		tools := llm.NewDefaultRegistry()
		if cfg.AgentHostCommands {
			log.Printf("WARNING: AGENT_HOST_COMMANDS=true — the agent can run shell commands on this host")
			tools = tools.WithHostCommands()
		}

		agents = llm.NewAgentRegistry(
			agent.NewFactory(
				problemStore,
				stream,
				eventStore,
				text,
				waiter,
				sandbox,
				tools,
			),
		)
	}

	// The tutor and its safety classifier use the configured local Ollama model.
	tipsHandler := tips.Handler{
		Pool:   pool,
		Filter: tips.NewDefaultFilter(),
	}
	if cfg.ChatEngine == "ollama" {
		tutorStream := llm.NewOllamaStream(
			cfg.OllamaBaseURL,
			cfg.OllamaModel,
			time.Duration(cfg.OllamaTimeoutSec)*time.Second,
		)
		tipsHandler.Agent = tips.New(tutorStream, cfg.OllamaModel)
		tipsHandler.Filter.Classifier = tips.NewOllamaClassifier(tutorStream)
	}

	log.Printf(
		"startup: env=%s chat=%s grader=%s tips=%v",
		cfg.Env,
		cfg.ChatEngine,
		engine.Name(),
		tipsHandler.Agent != nil,
	)

	modules := []httpx.Module{
		authRoutes(cfg, a.db),
		health{},
		account.Handler{Pool: pool},
		problems.Handler{Store: problemStore},
		sessions.Handler{Store: sessionStore, Problems: problemStore},
		submissions.Handler{
			Store:    submissions.Store{Pool: pool},
			Problems: problemStore,
			Sessions: sessionStore,
			Sandbox:  sandbox,
			Grading:  a.grading,
			Agents:   agentDropper{agents},
		},
		agent.Handler{
			Pool:      pool,
			Registry:  agents,
			Jailbreak: llm.NewChatJailbreakClassifier(),
			Waiter:    waiter,
			Sessions:  sessionStore,
			Events:    eventStore,
			Text:      text,
		},
		tipsHandler,
		replay.Handler{Pool: pool},
		profile.Handler{Pool: pool},
		forum.Handler{
			Pool:       pool,
			Cookie:     auth.CookieOptions{Domain: cfg.CookieDomain, Secure: cfg.CookieSecure},
			NewAccount: forum.DefaultNewAccountRules,
		},
		comments.Handler{Pool: pool},
		notes.Handler{Pool: pool},
		notifications.Handler{Pool: pool},
	}

	mux := http.NewServeMux()
	rt := httpx.NewRouter(mux, auth.Middleware(pool, cfg.CookieSecret))
	for _, m := range modules {
		m.Routes(rt)
	}
	a.handler = httpx.CORS(cfg.CORSOrigins)(httpx.Logging(mux))
	return nil
}

func authRoutes(cfg *config.Config, database *db.DB) auth.Routes {
	pool := database.Pool
	cookieOpts := auth.CookieOptions{Domain: cfg.CookieDomain, Secure: cfg.CookieSecure}
	states := auth.NewSQLOAuthStateStore(pool)
	users := auth.NewSQLUserStore(pool)
	sess := auth.NewCookieSessionManager(cfg.CookieSecret, cookieOpts)
	return auth.Routes{
		Password: auth.NewPasswordAuthHandler(pool, cfg.CookieSecret, cookieOpts),
		Google: auth.NewGoogleHandler(cfg.GoogleOAuthClientID, cfg.GoogleOAuthSecret, cfg.GoogleOAuthRedirectURL,
			states, users, sess),
		GitHub: auth.NewGitHubHandler(cfg.GitHubOAuthClientID, cfg.GitHubOAuthSecret, cfg.GitHubOAuthRedirectURL,
			states, users, sess),
		Email: auth.NewEmailHandler(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUser, cfg.SMTPPass, cfg.SMTPFrom,
			cfg.PublicURL, auth.NewSQLMagicLinkTokenStore(pool), users, sess),
		CookieOpts: cookieOpts,
	}
}

// agentDropper tolerates a nil registry (chat disabled).
type agentDropper struct{ r *llm.AgentRegistry }

func (d agentDropper) Drop(id uuid.UUID) {
	if d.r != nil {
		d.r.Drop(id)
	}
}

// health is the liveness endpoint.
type health struct{}

func (health) Routes(rt *httpx.Router) {
	rt.Bare("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		httpx.JSON(w, http.StatusOK, map[string]any{"status": "ok", "time": time.Now().UTC().Format(time.RFC3339)})
	})
}
