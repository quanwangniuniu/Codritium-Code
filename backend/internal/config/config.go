package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Env                    string
	Port                   string
	DatabaseURL            string
	AnthropicAPIKey        string
	GoogleAPIKey           string
	E2BAPIKey              string
	CookieSecret           string
	CookieDomain           string
	CookieSecure           bool
	GraderEngine           string
	ChatEngine             string
	OllamaBaseURL          string
	OllamaModel            string
	OllamaTimeoutSec       int
	ClaudeFallbackEnabled  bool
	GoogleOAuthClientID    string
	GoogleOAuthSecret      string
	GoogleOAuthRedirectURL string
	GitHubOAuthClientID    string
	GitHubOAuthSecret      string
	GitHubOAuthRedirectURL string
	SMTPHost               string
	SMTPPort               int
	SMTPUser               string
	SMTPPass               string
	SMTPFrom               string
	PublicURL              string
	CORSOrigins            []string
	// AgentHostCommands enables the agent's RunCommand tool, which executes
	// shell commands on this host. Off by default: see llm.WithHostCommands.
	AgentHostCommands bool
	Paths             Paths
}

func Load() (*Config, error) {
	paths, err := ResolvePaths()
	if err != nil {
		return nil, err
	}
	_ = godotenv.Load(filepath.Join(paths.Root, ".env"))
	_ = godotenv.Load(".env")
	// SANDBOX_PYTHON may come from .env, which was not loaded yet above.
	if v := os.Getenv("SANDBOX_PYTHON"); v != "" {
		paths.SandboxPython = v
	}

	c := &Config{
		Env:                    getenv("ENV", "dev"),
		Port:                   getenv("PORT", "8080"),
		DatabaseURL:            getenv("DATABASE_URL", "postgres://codritium:codritium@localhost:5432/codritium?sslmode=disable"),
		AnthropicAPIKey:        os.Getenv("ANTHROPIC_API_KEY"),
		GoogleAPIKey:           os.Getenv("GOOGLE_API_KEY"),
		E2BAPIKey:              os.Getenv("E2B_API_KEY"),
		CookieSecret:           os.Getenv("COOKIE_SECRET"),
		CookieDomain:           os.Getenv("COOKIE_DOMAIN"),
		GraderEngine:           getenv("GRADER_ENGINE", "gemini"),
		ChatEngine:             getenv("CHAT_ENGINE", "ollama"),
		OllamaBaseURL:          getenv("OLLAMA_BASE_URL", "http://localhost:11434"),
		OllamaModel:            getenv("OLLAMA_MODEL", "qwen3:8b"),
		OllamaTimeoutSec:       getenvInt("OLLAMA_TIMEOUT_SEC", 180),
		ClaudeFallbackEnabled:  getenvBool("CLAUDE_FALLBACK_ENABLED", false),
		GoogleOAuthClientID:    os.Getenv("GOOGLE_OAUTH_CLIENT_ID"),
		GoogleOAuthSecret:      os.Getenv("GOOGLE_OAUTH_SECRET"),
		GoogleOAuthRedirectURL: os.Getenv("GOOGLE_OAUTH_REDIRECT_URL"),
		GitHubOAuthClientID:    os.Getenv("GITHUB_OAUTH_CLIENT_ID"),
		GitHubOAuthSecret:      os.Getenv("GITHUB_OAUTH_SECRET"),
		GitHubOAuthRedirectURL: os.Getenv("GITHUB_OAUTH_REDIRECT_URL"),
		SMTPHost:               os.Getenv("SMTP_HOST"),
		SMTPPort:               getenvInt("SMTP_PORT", 587),
		SMTPUser:               os.Getenv("SMTP_USER"),
		SMTPPass:               os.Getenv("SMTP_PASS"),
		SMTPFrom:               os.Getenv("SMTP_FROM"),
		PublicURL:              getenv("PUBLIC_URL", "http://localhost:3000"),
		Paths:                  paths,
		AgentHostCommands:      getenvBool("AGENT_HOST_COMMANDS", false),
	}

	// The session cookie is Secure by default in prod. dev and staging may
	// run over plain HTTP, where a Secure cookie would be silently dropped.
	// COOKIE_SECURE overrides in either direction.
	c.CookieSecure = getenvBool("COOKIE_SECURE", c.Env == "prod")

	switch c.Env {
	case "dev", "staging", "prod":
	default:
		return nil, fmt.Errorf("ENV must be 'dev', 'staging', or 'prod', got %q", c.Env)
	}

	// HMAC session-cookie key. No default — a forgotten env var must crash the
	// server, never run with a guessable secret.
	if c.CookieSecret == "" {
		return nil, fmt.Errorf("COOKIE_SECRET is required (HMAC key for session cookies; set per environment)")
	}

	if c.E2BAPIKey == "" {
		return nil, fmt.Errorf("E2B_API_KEY is required")
	}

	// Engine enum validation.
	switch c.GraderEngine {
	case "ollama", "gemini", "anthropic":
	default:
		return nil, fmt.Errorf(
			"GRADER_ENGINE must be 'ollama', 'gemini', or 'anthropic', got %q",
			c.GraderEngine,
		)
	}

	switch c.ChatEngine {
	case "disabled", "ollama":
	default:
		return nil, fmt.Errorf(
			"CHAT_ENGINE must be 'disabled' or 'ollama', got %q",
			c.ChatEngine,
		)
	}

	if c.GraderEngine == "ollama" || c.ChatEngine == "ollama" {
		if c.OllamaBaseURL == "" {
			return nil, fmt.Errorf("OLLAMA_BASE_URL is required when Ollama chat or grading is enabled")
		}
		if c.OllamaModel == "" {
			return nil, fmt.Errorf("OLLAMA_MODEL is required when Ollama chat or grading is enabled")
		}
		if c.OllamaTimeoutSec <= 0 {
			return nil, fmt.Errorf("OLLAMA_TIMEOUT_SEC must be greater than zero")
		}
	}

	googleActive := c.GraderEngine == "gemini"
	if googleActive && c.GoogleAPIKey == "" {
		return nil, fmt.Errorf(
			"GOOGLE_API_KEY is required when Gemini grading is enabled",
		)
	}

	// The Claude grader is off by default: GRADER_ENGINE=anthropic must be
	// paired with CLAUDE_FALLBACK_ENABLED=true, which requires ANTHROPIC_API_KEY.
	if !c.ClaudeFallbackEnabled {
		if c.GraderEngine == "anthropic" {
			return nil, fmt.Errorf("GRADER_ENGINE=anthropic requires CLAUDE_FALLBACK_ENABLED=true")
		}
	}
	claudeActive := c.ClaudeFallbackEnabled || c.GraderEngine == "anthropic"
	if claudeActive && c.AnthropicAPIKey == "" {
		return nil, fmt.Errorf("ANTHROPIC_API_KEY is required when the Claude path is active")
	}

	// Browser origins allowed to call the API with cookies: CORS_ORIGINS
	// (comma-separated), else PUBLIC_URL plus the local dev ports.
	if v := os.Getenv("CORS_ORIGINS"); v != "" {
		for _, o := range strings.Split(v, ",") {
			if o = strings.TrimSpace(o); o != "" {
				c.CORSOrigins = append(c.CORSOrigins, o)
			}
		}
	} else {
		c.CORSOrigins = []string{c.PublicURL, "http://localhost:3000", "http://localhost:3012"}
	}

	return c, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getenvBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}

func getenvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}
