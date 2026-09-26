package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	Env                    string
	Port                   string
	DatabaseURL            string
	RedisURL               string
	AnthropicAPIKey        string
	GoogleAPIKey           string
	E2BAPIKey              string
	CookieSecret           string
	CookieDomain           string
	CookieSecure           bool
	GraderEngine           string
	ChatEngine             string
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
}

// IsDev reports whether the deployment is the local dev environment. Use to
// gate routes that exist solely to ease local iteration (mock login, fixture
// resetters) and must never be reachable in staging or prod.
func (c *Config) IsDev() bool { return c.Env == "dev" }

func Load() (*Config, error) {
	_ = godotenv.Load("../.env")
	_ = godotenv.Load(".env")

	c := &Config{
		Env:                    getenv("ENV", "dev"),
		Port:                   getenv("PORT", "8080"),
		DatabaseURL:            getenv("DATABASE_URL", "postgres://codritium:codritium@localhost:5434/codritium?sslmode=disable"),
		RedisURL:               getenv("REDIS_URL", "redis://localhost:6381"),
		AnthropicAPIKey:        os.Getenv("ANTHROPIC_API_KEY"),
		GoogleAPIKey:           os.Getenv("GOOGLE_API_KEY"),
		E2BAPIKey:              os.Getenv("E2B_API_KEY"),
		CookieSecret:           os.Getenv("COOKIE_SECRET"),
		CookieDomain:           os.Getenv("COOKIE_DOMAIN"),
		CookieSecure:           getenvBool("COOKIE_SECURE", false),
		GraderEngine:           getenv("GRADER_ENGINE", "gemini"),
		ChatEngine:             getenv("CHAT_ENGINE", "gemini"),
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
	}

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

	// GOOGLE_API_KEY is required: all default paths run on Gemini.
	if c.GoogleAPIKey == "" {
		return nil, fmt.Errorf("GOOGLE_API_KEY is required (default chat and grader engines are Gemini)")
	}
	if c.E2BAPIKey == "" {
		return nil, fmt.Errorf("E2B_API_KEY is required")
	}

	// Engine enum validation.
	switch c.GraderEngine {
	case "gemini", "anthropic":
	default:
		return nil, fmt.Errorf("GRADER_ENGINE must be 'gemini' or 'anthropic', got %q", c.GraderEngine)
	}
	switch c.ChatEngine {
	case "gemini", "anthropic":
	default:
		return nil, fmt.Errorf("CHAT_ENGINE must be 'gemini' or 'anthropic', got %q", c.ChatEngine)
	}

	// Claude path is preserved but disabled by default. CHAT_ENGINE=anthropic
	// or GRADER_ENGINE=anthropic must be paired with CLAUDE_FALLBACK_ENABLED=true,
	// and that combination then requires ANTHROPIC_API_KEY.
	if !c.ClaudeFallbackEnabled {
		if c.ChatEngine == "anthropic" {
			return nil, fmt.Errorf("CHAT_ENGINE=anthropic requires CLAUDE_FALLBACK_ENABLED=true")
		}
		if c.GraderEngine == "anthropic" {
			return nil, fmt.Errorf("GRADER_ENGINE=anthropic requires CLAUDE_FALLBACK_ENABLED=true")
		}
	}
	claudeActive := c.ClaudeFallbackEnabled || c.ChatEngine == "anthropic" || c.GraderEngine == "anthropic"
	if claudeActive && c.AnthropicAPIKey == "" {
		return nil, fmt.Errorf("ANTHROPIC_API_KEY is required when the Claude path is active")
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
