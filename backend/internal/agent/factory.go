package agent

import (
	"context"
	"fmt"
	"log"

	"github.com/google/uuid"

	"codritium/backend/internal/events"
	"codritium/backend/internal/grader"
	"codritium/backend/internal/llm"
	"codritium/backend/internal/problems"
)

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
// construct a fresh Agent the first time a candidate hits agent chat on
// a session. Loads the challenge's README / starter files / hidden
// test file path from the problems table; wires the engine-neutral
// stream client + DecisionWaiter + events store; locks the visible
// test path via DenyRule.
// NewFactory builds the AgentRegistry factory: a fresh Agent per session,
// with the problem's starter files as its workspace and the hidden test
// protected by deny rules.
func NewFactory(problemStore problems.Store, stream llm.LLMStreamClient, store *events.Store, text *llm.TextBroadcaster, waiter *llm.DecisionWaiter, sandbox grader.Sandbox, tools llm.Registry) llm.AgentFactory {
	return func(ctx context.Context, sessionID uuid.UUID, slug string) (*llm.Agent, error) {
		prob, err := problemStore.Get(ctx, slug)
		if err != nil {
			return nil, err
		}
		readme, hiddenTestFile := prob.ReadmeMD, prob.HiddenTestFile
		files := prob.Starter.Variant(problems.VariantAsIs)

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
			Tools:        tools,
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
	return "You are the candidate-agent inside Codritium. The user is a software engineering candidate" +
		" working on the following problem. When the user asks you to fix or implement something, use the" +
		" FileEdit tool to propose the required changes. FileEdit waits for candidate approval, so calling" +
		" it does not bypass review. Do not provide an Apply code block instead of calling FileEdit.\n\n" +
		"Problem README:\n" + readme + "\n\n" +
		"Workspace files (visible):\n" + fileList + "\n" +
		"For large or multi-file workspaces, begin with one discovery phase: call Glob only when the visible" +
		" workspace file list is insufficient, then use Grep to locate relevant symbols and line numbers." +
		" Do not call Glob again before each FileRead. After identifying relevant files, read only those" +
		" files. Use start_line and line_count for required sections of long files, and avoid requesting" +
		" file ranges that are already available in the current context." +
		" Use FileEdit with path, exact old_text, and new_text to replace one focused code block." +
		" Include enough surrounding context for old_text to match exactly once, but do not resend the entire file." +
		" Use new_text without old_text only when creating a new file." +
		" Invoke FileEdit through the tool interface; never print or describe FileEdit JSON in assistant text.\n\n" +
		"There is no pre-supplied test file. Before calling RunTests, create a pytest file such as" +
		" `test_my.py` through FileEdit and wait for that edit to be approved. Never call RunTests with a path" +
		" that does not already exist in the visible workspace. If RunTests reports that the file is missing," +
		" create it through FileEdit and retry RunTests. The hidden test " + hiddenTest +
		" is invisible to you and protected from access — do not try to read or modify it."
}
