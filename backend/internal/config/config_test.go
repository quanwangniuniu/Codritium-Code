package config

import (
	"strings"
	"testing"
)

func setBaseTestEnv(t *testing.T) {
	t.Helper()

	t.Setenv("ENV", "dev")
	t.Setenv("COOKIE_SECRET", "test-cookie-secret-at-least-32-bytes")
	t.Setenv("E2B_API_KEY", "test-e2b-key")
	t.Setenv("GOOGLE_API_KEY", "")
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("CLAUDE_FALLBACK_ENABLED", "false")
	t.Setenv("GRADER_ENGINE", "ollama")
	t.Setenv("CHAT_ENGINE", "disabled")
	t.Setenv("OLLAMA_BASE_URL", "http://localhost:11434")
	t.Setenv("OLLAMA_MODEL", "qwen3:8b")
	t.Setenv("OLLAMA_TIMEOUT_SEC", "180")
}

func TestLoadOllamaWithoutCloudKeys(t *testing.T) {
	setBaseTestEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.GraderEngine != "ollama" {
		t.Fatalf("GraderEngine = %q, want ollama", cfg.GraderEngine)
	}
	if cfg.ChatEngine != "disabled" {
		t.Fatalf("ChatEngine = %q, want disabled", cfg.ChatEngine)
	}
	if cfg.OllamaBaseURL != "http://localhost:11434" {
		t.Fatalf("OllamaBaseURL = %q", cfg.OllamaBaseURL)
	}
	if cfg.OllamaModel != "qwen3:8b" {
		t.Fatalf("OllamaModel = %q", cfg.OllamaModel)
	}
	if cfg.OllamaTimeoutSec != 180 {
		t.Fatalf("OllamaTimeoutSec = %d, want 180", cfg.OllamaTimeoutSec)
	}
}

func TestLoadGeminiRequiresGoogleKey(t *testing.T) {
	setBaseTestEnv(t)
	t.Setenv("GRADER_ENGINE", "gemini")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want GOOGLE_API_KEY error")
	}
	if !strings.Contains(err.Error(), "GOOGLE_API_KEY") {
		t.Fatalf("Load() error = %q, want GOOGLE_API_KEY error", err)
	}
}

func TestLoadAnthropicRequiresAnthropicKey(t *testing.T) {
	setBaseTestEnv(t)
	t.Setenv("GRADER_ENGINE", "anthropic")
	t.Setenv("CLAUDE_FALLBACK_ENABLED", "true")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want ANTHROPIC_API_KEY error")
	}
	if !strings.Contains(err.Error(), "ANTHROPIC_API_KEY") {
		t.Fatalf("Load() error = %q, want ANTHROPIC_API_KEY error", err)
	}
}

func TestLoadRejectsUnknownGraderEngine(t *testing.T) {
	setBaseTestEnv(t)
	t.Setenv("GRADER_ENGINE", "unknown")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want invalid grader engine error")
	}
	if !strings.Contains(err.Error(), "GRADER_ENGINE") {
		t.Fatalf("Load() error = %q, want GRADER_ENGINE error", err)
	}
}

func TestLoadCookieSecureDefaultsByEnv(t *testing.T) {
	for _, tc := range []struct {
		env, override string
		want          bool
	}{
		{"dev", "", false},
		{"staging", "", false},
		{"staging", "true", true},
		{"prod", "", true},
		{"prod", "false", false},
		{"dev", "true", true},
	} {
		setBaseTestEnv(t)
		t.Setenv("ENV", tc.env)
		t.Setenv("COOKIE_SECURE", tc.override)

		cfg, err := Load()
		if err != nil {
			t.Fatalf("ENV=%s COOKIE_SECURE=%q: Load() error = %v", tc.env, tc.override, err)
		}
		if cfg.CookieSecure != tc.want {
			t.Fatalf("ENV=%s COOKIE_SECURE=%q: CookieSecure = %v, want %v",
				tc.env, tc.override, cfg.CookieSecure, tc.want)
		}
	}
}
