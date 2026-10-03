package llm

import (
	"context"
	"strings"
	"testing"
)

func TestFileEditTargetedReplacement(t *testing.T) {
	workspace := &Workspace{
		Files: map[string]string{
			"main.py": "before\nold value\nafter\n",
		},
	}

	output, isError, err := (fileEditTool{}).Execute(
		context.Background(),
		map[string]any{
			"path":     "main.py",
			"old_text": "old value",
			"new_text": "new value",
		},
		ToolDeps{Workspace: workspace},
	)

	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if isError {
		t.Fatalf("Execute returned tool error: %s", output)
	}

	want := "before\nnew value\nafter\n"
	if got := workspace.Files["main.py"]; got != want {
		t.Fatalf("file content = %q, want %q", got, want)
	}
}

func TestFileEditRejectsAmbiguousReplacement(t *testing.T) {
	const original = "value\nvalue\n"

	workspace := &Workspace{
		Files: map[string]string{
			"main.py": original,
		},
	}

	output, isError, err := (fileEditTool{}).Execute(
		context.Background(),
		map[string]any{
			"path":     "main.py",
			"old_text": "value",
			"new_text": "changed",
		},
		ToolDeps{Workspace: workspace},
	)

	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !isError {
		t.Fatalf("expected ambiguous replacement to fail")
	}
	if !strings.Contains(output, "more than once") {
		t.Fatalf("unexpected tool output: %s", output)
	}
	if got := workspace.Files["main.py"]; got != original {
		t.Fatalf("file changed after rejected edit: %q", got)
	}
}

func TestFileEditCreatesNewFile(t *testing.T) {
	workspace := &Workspace{
		Files: map[string]string{},
	}

	output, isError, err := (fileEditTool{}).Execute(
		context.Background(),
		map[string]any{
			"path":     "test_main.py",
			"new_text": "def test_main():\n    assert True\n",
		},
		ToolDeps{Workspace: workspace},
	)

	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if isError {
		t.Fatalf("Execute returned tool error: %s", output)
	}
	if got := workspace.Files["test_main.py"]; got == "" {
		t.Fatal("new file was not created")
	}
}
