package llm

import (
	"strings"
	"testing"
)

func TestContextManager_TracksFileRanges(t *testing.T) {
	manager := NewContextManager()
	workspace := &Workspace{
		Files: map[string]string{
			"app.py": "one\ntwo\nthree\nfour",
		},
	}
	input := map[string]any{
		"path":       "app.py",
		"start_line": float64(2),
		"line_count": float64(2),
	}

	if manager.AlreadyRead(input, workspace) {
		t.Fatal("range should not be recorded before first read")
	}

	manager.RememberRead(input, workspace)

	if !manager.AlreadyRead(input, workspace) {
		t.Fatal("same file range should be recorded")
	}

	differentRange := map[string]any{
		"path":       "app.py",
		"start_line": float64(3),
		"line_count": float64(2),
	}
	if manager.AlreadyRead(differentRange, workspace) {
		t.Fatal("different range must not be treated as repeated")
	}
}

func TestContextManager_AllowsReadAfterFileChanges(t *testing.T) {
	manager := NewContextManager()
	workspace := &Workspace{
		Files: map[string]string{
			"app.py": "old content",
		},
	}
	input := map[string]any{"path": "app.py"}

	manager.RememberRead(input, workspace)
	if !manager.AlreadyRead(input, workspace) {
		t.Fatal("initial read was not recorded")
	}

	workspace.Set("app.py", "new content")

	if manager.AlreadyRead(input, workspace) {
		t.Fatal("changed file should be readable again")
	}
}

func TestContextManager_CompactsLongHistory(t *testing.T) {
	manager := NewContextManager()
	manager.BudgetTokens = 100
	manager.KeepRecentMessages = 2

	messages := []Message{
		{
			Role: "user",
			Content: []ContentBlock{{
				Kind: "text",
				Text: strings.Repeat("old goal ", 100),
			}},
		},
		{
			Role: "assistant",
			Content: []ContentBlock{{
				Kind: "text",
				Text: strings.Repeat("old explanation ", 100),
			}},
		},
		{
			Role: "user",
			Content: []ContentBlock{{
				Kind: "text",
				Text: "latest request",
			}},
		},
		{
			Role: "assistant",
			Content: []ContentBlock{{
				Kind: "text",
				Text: "latest answer",
			}},
		},
	}

	compacted, stats := manager.Prepare(messages)
	if stats == nil {
		t.Fatal("expected context compaction")
	}
	if stats.TokensAfter > manager.BudgetTokens {
		t.Fatalf(
			"tokens after compaction = %d, budget = %d",
			stats.TokensAfter,
			manager.BudgetTokens,
		)
	}
	if stats.RemovedMessages != 2 {
		t.Fatalf(
			"removed messages = %d, want 2",
			stats.RemovedMessages,
		)
	}
	if len(compacted) != 3 {
		t.Fatalf(
			"compacted messages = %d, want 3",
			len(compacted),
		)
	}
	if !strings.Contains(
		compacted[0].Content[0].Text,
		"Compacted context summary",
	) {
		t.Fatal("compacted summary is missing")
	}
	if compacted[1].Content[0].Text != "latest request" {
		t.Fatal("latest request was not preserved")
	}
}

func TestContextManager_DoesNotCompactWithinBudget(t *testing.T) {
	manager := NewContextManager()
	manager.BudgetTokens = 1000

	messages := []Message{
		{
			Role: "user",
			Content: []ContentBlock{{
				Kind: "text",
				Text: "small request",
			}},
		},
	}

	prepared, stats := manager.Prepare(messages)
	if stats != nil {
		t.Fatal("unexpected compaction")
	}
	if len(prepared) != 1 {
		t.Fatalf("prepared messages = %d, want 1", len(prepared))
	}
}

func TestContextManager_TracksRepeatedSearches(t *testing.T) {
	manager := NewContextManager()
	workspace := &Workspace{
		Files: map[string]string{
			"main.py": "print('hello')",
		},
	}
	input := map[string]any{
		"pattern": "*.py",
	}

	if manager.AlreadySearched("Glob", input, workspace) {
		t.Fatal("search unexpectedly recorded")
	}

	manager.RememberSearch("Glob", input, workspace)

	if !manager.AlreadySearched("Glob", input, workspace) {
		t.Fatal("expected repeated search to be detected")
	}

	workspace.Files["other.py"] = "value = 1"

	if manager.AlreadySearched("Glob", input, workspace) {
		t.Fatal("search should be allowed after workspace changes")
	}
}
