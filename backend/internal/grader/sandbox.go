package grader

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

type SandboxInput struct {
	StarterFiles       map[string]string `json:"starter_files"`
	CandidateFiles     map[string]string `json:"candidate_files"`
	HiddenTestFilename string            `json:"hidden_test_filename"`
	HiddenTestContent  string            `json:"hidden_test_content"`
	TimeoutSec         int               `json:"timeout_sec"`
}

type TestResult struct {
	Name    string `json:"name"`
	Outcome string `json:"outcome"`
	Message string `json:"message,omitempty"`
}

type SandboxOutput struct {
	Status      string       `json:"status"`
	TestResults []TestResult `json:"test_results"`
	PassCount   int          `json:"pass_count"`
	FailCount   int          `json:"fail_count"`
	Total       int          `json:"total"`
	ExitCode    int          `json:"exit_code"`
	Stdout      string       `json:"stdout"`
	Stderr      string       `json:"stderr"`
	DurationSec float64      `json:"duration_sec"`
	Error       string       `json:"error,omitempty"`
}

// Sandbox runs candidate code through the E2B-backed Python wrapper
// (sandbox/run_pytest.py). Paths come from config.Paths.
type Sandbox struct {
	Python string // interpreter with the e2b package installed
	Script string // path to run_pytest.py
}

// RunPytest invokes the wrapper with input on stdin and parses its JSON.
func (s Sandbox) RunPytest(ctx context.Context, input SandboxInput) (*SandboxOutput, error) {
	if input.TimeoutSec == 0 {
		input.TimeoutSec = 60
	}
	body, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}

	pythonBin, scriptPath := s.Python, s.Script

	cctx, cancel := context.WithTimeout(ctx, time.Duration(input.TimeoutSec+90)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cctx, pythonBin, scriptPath)
	cmd.Stdin = strings.NewReader(string(body))
	// Explicitly inherit env so the subprocess sees godotenv-loaded keys.
	cmd.Env = os.Environ()

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	runErr := cmd.Run()

	// Parse stdout even on non-zero exit — the wrapper writes structured JSON errors there.
	out := stdout.Bytes()
	if len(out) > 0 {
		var result SandboxOutput
		if err := json.Unmarshal(out, &result); err == nil {
			if runErr != nil && result.Error == "" {
				result.Error = fmt.Sprintf("subprocess exit: %v stderr=%s", runErr, stderr.String())
			}
			if result.Status == "error" {
				return &result, fmt.Errorf("sandbox: %s", result.Error)
			}
			return &result, nil
		}
	}
	return nil, fmt.Errorf("python sandbox run: %w stderr=%s stdout=%s",
		runErr, stderr.String(), truncate(string(out), 500))
}
