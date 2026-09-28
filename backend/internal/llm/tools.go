package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Tool is the engine-neutral contract for one V0 tool. The main loop
// looks tools up by Name, advertises Spec() to the model, and routes
// approved tool_use blocks to Execute().
//
// V0 ships exactly three tools: FileRead, FileEdit, RunTests. Bash and
// WebFetch are intentionally excluded (HANDOFF §4 + agent_design.md §8.1).
type Tool interface {
	Name() string
	Spec() ToolSpec
	Execute(ctx context.Context, input map[string]any, deps ToolDeps) (output string, isError bool, err error)
}

// Workspace is the candidate's in-memory file tree for one session.
// FileEdit mutates Files; FileRead reads it. The Workspace is per-session
// and lives in the chat goroutine — never shared across sessions.
type Workspace struct {
	Files map[string]string // path -> contents
}

// SandboxRunner runs the visible test suite (V0: pytest) against the
// current workspace state. The real implementation goes through E2B.
type SandboxRunner interface {
	RunPytest(ctx context.Context, files map[string]string, testFile string) (SandboxResult, error)
}

type SandboxResult struct {
	Passed           int      `json:"passed"`
	Failed           int      `json:"failed"`
	VisibleTestNames []string `json:"visible_test_names"`
	DurationMs       int64    `json:"duration_ms"`
	Stdout           string   `json:"stdout,omitempty"`
}

// DenyRule blocks a tool / path combination for one challenge. Loaded from
// the seed JSON; checked at the top of Execute. The canonical example is
// "block FileEdit on test_answer.py" so the candidate can't have the
// agent rewrite the visible-test oracle.
type DenyRule struct {
	Tool        string `json:"tool"`         // exact Name() match
	PathPattern string `json:"path_pattern"` // exact-suffix match for V0
	Reason      string `json:"reason"`
}

// ToolDeps is the per-turn carrier handed to every Tool.Execute call.
type ToolDeps struct {
	Workspace *Workspace
	Sandbox   SandboxRunner
	Deny      []DenyRule
	// VisibleTestFile is the challenge's visible_test_file (e.g.
	// "test_answer.py"). RunTests uses it; FileEdit / FileRead consult
	// Deny to forbid touching it through FileEdit.
	VisibleTestFile string
	// TmpFS is the per-session scratch directory the Grep / Glob /
	// RunCommand tools read and write under. Nil disables those tools
	// gracefully.
	TmpFS *SessionTmpFS
}

// Registry is a Name → Tool map used by the main loop.
type Registry map[string]Tool

// NewDefaultRegistry returns the active tool set. FileRead / FileEdit /
// RunTests are the original three. Grep / Glob / RunCommand were added in
// 1.0 to give the agent richer code exploration plus an escape hatch into
// a per-session shell.
func NewDefaultRegistry() Registry {
	return Registry{
		"FileRead":   fileReadTool{},
		"FileEdit":   fileEditTool{},
		"RunTests":   runTestsTool{},
		"Grep":       grepTool{},
		"Glob":       globTool{},
		"RunCommand": runCommandTool{},
	}
}

// ErrDenied is returned by Execute when a deny rule matches. The main loop
// turns this into a tool_result with is_error=true; it does NOT abort the
// turn (the model can still adapt).
var ErrDenied = errors.New("tool: denied by challenge rule")

func checkDeny(toolName, path string, rules []DenyRule) error {
	for _, r := range rules {
		if r.Tool != toolName {
			continue
		}
		if r.PathPattern != "" && strings.HasSuffix(path, r.PathPattern) {
			return fmt.Errorf("%w: %s on %s (%s)", ErrDenied, toolName, path, r.Reason)
		}
	}
	return nil
}

// ─── FileRead ───────────────────────────────────────────────────────────

type fileReadTool struct{}

func (fileReadTool) Name() string { return "FileRead" }

func (fileReadTool) Spec() ToolSpec {
	return ToolSpec{
		Name:        "FileRead",
		Description: "Read the full contents of a file in the workspace.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Workspace-relative file path.",
				},
			},
			"required": []string{"path"},
		},
	}
}

func (fileReadTool) Execute(ctx context.Context, input map[string]any, deps ToolDeps) (string, bool, error) {
	path, ok := input["path"].(string)
	if !ok || path == "" {
		return `{"error":"path required"}`, true, nil
	}
	content, ok := deps.Workspace.Files[path]
	if !ok {
		return fmt.Sprintf(`{"error":"file not found: %s"}`, path), true, nil
	}
	return content, false, nil
}

// ─── FileEdit ───────────────────────────────────────────────────────────

type fileEditTool struct{}

func (fileEditTool) Name() string { return "FileEdit" }

func (fileEditTool) Spec() ToolSpec {
	return ToolSpec{
		Name: "FileEdit",
		Description: "Propose a replacement for a file in the workspace. The candidate must " +
			"approve (and may modify) the patch before it lands.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Workspace-relative file path.",
				},
				"content": map[string]any{
					"type":        "string",
					"description": "Full replacement contents of the file.",
				},
			},
			"required": []string{"path", "content"},
		},
	}
}

func (fileEditTool) Execute(ctx context.Context, input map[string]any, deps ToolDeps) (string, bool, error) {
	path, _ := input["path"].(string)
	content, _ := input["content"].(string)
	if path == "" {
		return `{"error":"path required"}`, true, nil
	}
	if err := checkDeny("FileEdit", path, deps.Deny); err != nil {
		return fmt.Sprintf(`{"error":%q}`, err.Error()), true, nil
	}
	deps.Workspace.Files[path] = content
	resp, _ := json.Marshal(map[string]any{
		"applied": true,
		"path":    path,
		"size":    len(content),
	})
	return string(resp), false, nil
}

// ─── RunTests ───────────────────────────────────────────────────────────

type runTestsTool struct{}

func (runTestsTool) Name() string { return "RunTests" }

func (runTestsTool) Spec() ToolSpec {
	return ToolSpec{
		Name:        "RunTests",
		Description: "Run a pytest test file that exists in the workspace. The candidate (or you, via FileEdit) must have created it first. Specify the exact workspace path.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Workspace-relative path of the pytest file to run, e.g. test_my.py.",
				},
			},
			"required":             []string{"path"},
			"additionalProperties": false,
		},
	}
}

func (runTestsTool) Execute(ctx context.Context, input map[string]any, deps ToolDeps) (string, bool, error) {
	if deps.Sandbox == nil {
		return `{"error":"sandbox not configured"}`, true, nil
	}
	path, ok := input["path"].(string)
	if !ok || path == "" {
		return `{"error":"path required"}`, true, nil
	}
	if _, exists := deps.Workspace.Files[path]; !exists {
		return fmt.Sprintf(`{"error":"test file %q not found in workspace; create it via FileEdit first"}`, path), true, nil
	}
	start := time.Now()
	res, err := deps.Sandbox.RunPytest(ctx, deps.Workspace.Files, path)
	if err != nil {
		return fmt.Sprintf(`{"error":%q}`, err.Error()), true, nil
	}
	if res.DurationMs == 0 {
		res.DurationMs = time.Since(start).Milliseconds()
	}
	out, _ := json.Marshal(res)
	return string(out), false, nil
}

// ─── Grep ──────────────────────────────────────────────────────────────

type grepTool struct{}

func (grepTool) Name() string { return "Grep" }

func (grepTool) Spec() ToolSpec {
	return ToolSpec{
		Name:        "Grep",
		Description: "Regex search over workspace files. Returns matching lines with file path and line number.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"pattern":   map[string]any{"type": "string", "description": "Regular expression to search for."},
				"path_glob": map[string]any{"type": "string", "description": "Optional filename glob filter (e.g. '*.py')."},
			},
			"required": []string{"pattern"},
		},
	}
}

const grepMaxMatches = 200

type grepMatch struct {
	File string `json:"file"`
	Line int    `json:"line"`
	Text string `json:"text"`
}

func (grepTool) Execute(ctx context.Context, input map[string]any, deps ToolDeps) (string, bool, error) {
	pattern, _ := input["pattern"].(string)
	if pattern == "" {
		return `{"error":"pattern required"}`, true, nil
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Sprintf(`{"error":"invalid regex: %s"}`, err.Error()), true, nil
	}
	pathGlob, _ := input["path_glob"].(string)
	if deps.Workspace == nil {
		return `[]`, false, nil
	}
	matches := make([]grepMatch, 0)
	for name, content := range deps.Workspace.Files {
		if err := checkDeny("Grep", name, deps.Deny); err != nil {
			continue
		}
		if pathGlob != "" {
			ok, _ := filepath.Match(pathGlob, filepath.Base(name))
			if !ok {
				continue
			}
		}
		for i, line := range strings.Split(content, "\n") {
			if re.MatchString(line) {
				matches = append(matches, grepMatch{File: name, Line: i + 1, Text: line})
				if len(matches) >= grepMaxMatches {
					break
				}
			}
		}
		if len(matches) >= grepMaxMatches {
			break
		}
	}
	out, _ := json.Marshal(matches)
	return string(out), false, nil
}

// ─── Glob ──────────────────────────────────────────────────────────────

type globTool struct{}

func (globTool) Name() string { return "Glob" }

func (globTool) Spec() ToolSpec {
	return ToolSpec{
		Name:        "Glob",
		Description: "List workspace files whose name matches the given glob pattern.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"pattern": map[string]any{"type": "string", "description": "Glob pattern, e.g. '*.py'."},
			},
			"required": []string{"pattern"},
		},
	}
}

func (globTool) Execute(ctx context.Context, input map[string]any, deps ToolDeps) (string, bool, error) {
	pattern, _ := input["pattern"].(string)
	if pattern == "" {
		return `{"error":"pattern required"}`, true, nil
	}
	if deps.Workspace == nil {
		return `[]`, false, nil
	}
	out := make([]string, 0)
	for name := range deps.Workspace.Files {
		if err := checkDeny("Glob", name, deps.Deny); err != nil {
			continue
		}
		base := filepath.Base(name)
		ok, _ := filepath.Match(pattern, base)
		if !ok {
			ok, _ = filepath.Match(pattern, name)
		}
		if ok {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	result, _ := json.Marshal(out)
	return string(result), false, nil
}

// ─── RunCommand ────────────────────────────────────────────────────────

type runCommandTool struct{}

func (runCommandTool) Name() string { return "RunCommand" }

func (runCommandTool) Spec() ToolSpec {
	return ToolSpec{
		Name:        "RunCommand",
		Description: "Run a shell command in the session's per-candidate scratch directory. The workspace is materialized on each call; the hidden test file is never written. Network is intentionally off-limits.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"command": map[string]any{"type": "string", "description": "Single shell command line."},
			},
			"required": []string{"command"},
		},
	}
}

const runCommandTimeout = 10 * time.Second

func (runCommandTool) Execute(ctx context.Context, input map[string]any, deps ToolDeps) (string, bool, error) {
	cmdStr, _ := input["command"].(string)
	if cmdStr == "" {
		return `{"error":"command required"}`, true, nil
	}
	if deps.TmpFS == nil {
		return `{"error":"tmpfs not configured for this session"}`, true, nil
	}
	if deps.Workspace != nil {
		if err := deps.TmpFS.Sync(deps.Workspace.Files); err != nil {
			return fmt.Sprintf(`{"error":"sync workspace: %s"}`, err.Error()), true, nil
		}
	}
	runCtx, cancel := context.WithTimeout(ctx, runCommandTimeout)
	defer cancel()
	cmd := exec.CommandContext(runCtx, "bash", "-c", cmdStr)
	cmd.Dir = deps.TmpFS.Dir()
	cmd.Env = []string{
		"PATH=/usr/local/bin:/usr/bin:/bin",
		"HOME=" + deps.TmpFS.Dir(),
		"LC_ALL=C.UTF-8",
	}
	out, _ := cmd.CombinedOutput()
	exitCode := -1
	if cmd.ProcessState != nil {
		exitCode = cmd.ProcessState.ExitCode()
	}
	result := map[string]any{
		"exit_code": exitCode,
		"stdout":    string(out),
		"timed_out": runCtx.Err() == context.DeadlineExceeded,
	}
	j, _ := json.Marshal(result)
	isErr := exitCode != 0
	return string(j), isErr, nil
}
