// cmd/grade is a backend-direct grader CLI. It bypasses HTTP and DB entirely,
// composing GraderInput from a problem JSON and a tier directory on disk,
// then invoking the chosen grader engine and printing the JSON result.
//
// Usage:
//
//	go run ./cmd/grade -tier poor -engine gemini \
//	    -problem ../seed/problems/22-build-rate-limiter-middleware.json \
//	    -tier-dir /Users/johns3248/project/AICH/temp/22 \
//	    [-skip-sandbox] [-out result.json]
//
// Notes:
//   - Engine selector overrides GRADER_ENGINE env var.
//   - Sandbox is run by default; set -skip-sandbox to score code+history only.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"google.golang.org/genai"

	"codritium/backend/internal/anthropic"
	"codritium/backend/internal/config"
	"codritium/backend/internal/grader"
)

type problemSeed struct {
	Slug               string                       `json:"slug"`
	Title              string                       `json:"title"`
	Difficulty         string                       `json:"difficulty"`
	ReadmeMD           string                       `json:"readme_md"`
	StarterFiles       map[string]map[string]string `json:"starter_files"`
	HiddenTestFilename string                       `json:"hidden_test_filename"`
	HiddenTestContent  string                       `json:"hidden_test_content"`
}

// flattenStarter selects one variant per file, preferring `as-is`.
func flattenStarter(all map[string]map[string]string) map[string]string {
	out := make(map[string]string, len(all))
	for name, variants := range all {
		if c, ok := variants["as-is"]; ok {
			out[name] = c
			continue
		}
		// Fall back to any single variant.
		for _, c := range variants {
			out[name] = c
			break
		}
	}
	return out
}

func main() {
	var (
		tier        = flag.String("tier", "", "tier name (poor|mid|good) — folder under -tier-dir")
		engine      = flag.String("engine", "", "grader engine: ollama|anthropic|gemini (overrides GRADER_ENGINE)")
		problemPath = flag.String("problem", "", "path to problem seed JSON (default: seed/problems/22-build-rate-limiter-middleware.json)")
		tierDir     = flag.String("tier-dir", "", "root dir containing poor/, mid/, good/ (required)")
		skipSandbox = flag.Bool("skip-sandbox", false, "skip E2B sandbox run; test_results will be null")
		outPath     = flag.String("out", "", "write JSON result here; if empty, write to stdout")
	)
	flag.Parse()

	if *tier == "" {
		log.Fatal("missing -tier")
	}
	if *tierDir == "" {
		log.Fatal("missing -tier-dir")
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	if *engine != "" {
		cfg.GraderEngine = *engine
	}
	if *problemPath == "" {
		*problemPath = filepath.Join(cfg.Paths.ProblemsSeed, "22-build-rate-limiter-middleware.json")
	}
	if cfg.GraderEngine != "ollama" &&
		cfg.GraderEngine != "anthropic" &&
		cfg.GraderEngine != "gemini" {
		log.Fatalf(
			"invalid engine: %q (use ollama|anthropic|gemini)",
			cfg.GraderEngine,
		)
	}
	if cfg.GraderEngine == "gemini" && cfg.GoogleAPIKey == "" {
		log.Fatal("GOOGLE_API_KEY required for engine=gemini")
	}

	ps, err := loadProblem(*problemPath)
	if err != nil {
		log.Fatalf("load problem: %v", err)
	}

	starter := flattenStarter(ps.StarterFiles)
	tierPath := filepath.Join(*tierDir, *tier)
	candidate, err := loadCandidateFiles(starter, tierPath)
	if err != nil {
		log.Fatalf("load candidate: %v", err)
	}

	history, err := parsePromptHistory(filepath.Join(tierPath, "prompt_history.txt"))
	if err != nil {
		log.Fatalf("parse history: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()

	var sandbox *grader.SandboxOutput
	if !*skipSandbox {
		log.Printf("[grade] running sandbox for tier=%s ...", *tier)
		t0 := time.Now()
		sandbox, err = grader.Sandbox{Python: cfg.Paths.SandboxPython, Script: cfg.Paths.SandboxScript}.RunPytest(ctx, grader.SandboxInput{
			StarterFiles:       starter,
			CandidateFiles:     candidate,
			HiddenTestFilename: ps.HiddenTestFilename,
			HiddenTestContent:  ps.HiddenTestContent,
			TimeoutSec:         90,
		})
		if err != nil {
			log.Fatalf("sandbox: %v", err)
		}
		log.Printf("[grade] sandbox done in %.1fs: %d/%d passed",
			time.Since(t0).Seconds(), sandbox.PassCount, sandbox.Total)
	}

	input := grader.GraderInput{
		ProblemTitle:      ps.Title,
		ProblemDifficulty: ps.Difficulty,
		ProblemReadme:     ps.ReadmeMD,
		StarterFiles:      starter,
		CandidateFiles:    candidate,
		PromptHistory:     history,
		TestResults:       sandbox,
	}

	log.Printf("[grade] engine=%s — calling grader ...", cfg.GraderEngine)
	t0 := time.Now()
	var result *grader.GraderResult
	switch cfg.GraderEngine {
	case "ollama":
		client := grader.NewOllamaClient(
			cfg.OllamaBaseURL,
			cfg.OllamaModel,
			time.Duration(cfg.OllamaTimeoutSec)*time.Second,
		)
		result, err = grader.RunOllama(ctx, client, input)

	case "gemini":
		client, clientErr := genai.NewClient(ctx, &genai.ClientConfig{
			APIKey:  cfg.GoogleAPIKey,
			Backend: genai.BackendGeminiAPI,
		})
		if clientErr != nil {
			log.Fatalf("gemini client: %v", clientErr)
		}
		result, err = grader.RunGemini(ctx, client, input)

	case "anthropic":
		client := anthropic.New(cfg.AnthropicAPIKey)
		result, err = grader.Run(ctx, client, input)
	}

	if err != nil {
		log.Fatalf("grader: %v", err)
	}
	log.Printf("[grade] grader done in %.1fs: final_score=%.1f", time.Since(t0).Seconds(), result.FinalScore)

	out := map[string]interface{}{
		"tier":         *tier,
		"engine":       cfg.GraderEngine,
		"problem_slug": ps.Slug,
		"sandbox":      sandbox,
		"grader":       result,
		"timestamp":    time.Now().Format(time.RFC3339),
	}
	body, _ := json.MarshalIndent(out, "", "  ")
	if *outPath != "" {
		if err := os.WriteFile(*outPath, body, 0o644); err != nil {
			log.Fatalf("write out: %v", err)
		}
		log.Printf("[grade] wrote %s (%d bytes)", *outPath, len(body))
	} else {
		fmt.Println(string(body))
	}
}

func loadProblem(path string) (*problemSeed, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var ps problemSeed
	if err := json.Unmarshal(body, &ps); err != nil {
		return nil, err
	}
	return &ps, nil
}

// loadCandidateFiles composes the candidate's final file set:
// starter files overridden by anything under <tierPath>/fixed/.
func loadCandidateFiles(starter map[string]string, tierPath string) (map[string]string, error) {
	out := make(map[string]string, len(starter))
	for k, v := range starter {
		out[k] = v
	}
	fixedDir := filepath.Join(tierPath, "fixed")
	entries, err := os.ReadDir(fixedDir)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", fixedDir, err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		body, err := os.ReadFile(filepath.Join(fixedDir, e.Name()))
		if err != nil {
			return nil, err
		}
		out[e.Name()] = string(body)
	}
	return out, nil
}

// parsePromptHistory matches the format produced by /temp/22/<tier>/prompt_history.txt:
//
//	=== Conversation export ===
//	model: ...
//	exported: ...
//
//	USER:
//	<text>
//
//	A:
//	<text>
//
//	=== end ===
func parsePromptHistory(path string) ([]grader.PromptHistoryItem, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var out []grader.PromptHistoryItem
	var role string
	var buf []string

	flush := func() {
		if role == "" {
			return
		}
		content := strings.TrimSpace(strings.Join(buf, "\n"))
		if content != "" {
			out = append(out, grader.PromptHistoryItem{Role: role, Content: content})
		}
		buf = nil
	}

	for _, line := range strings.Split(string(body), "\n") {
		s := strings.TrimSpace(line)
		switch {
		case s == "=== Conversation export ===":
			continue
		case s == "=== end ===":
			flush()
			return out, nil
		case strings.HasPrefix(line, "model:"), strings.HasPrefix(line, "exported:"):
			continue
		case s == "USER:" || strings.HasPrefix(line, "USER:"):
			flush()
			role = "user"
			continue
		case s == "A:" || strings.HasPrefix(line, "A:"):
			flush()
			role = "assistant"
			continue
		}
		if role != "" {
			buf = append(buf, line)
		}
	}
	flush()
	return out, nil
}
