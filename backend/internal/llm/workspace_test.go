package llm

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
)

// Without Workspace's lock, the runtime aborts with "concurrent map writes"
// here — this mirrors the chat handler syncing editor files while a turn's
// tools read and edit the same workspace.
func TestWorkspace_ConcurrentAccess(t *testing.T) {
	ws := &Workspace{Files: map[string]string{"a.py": "x"}}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(3)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 500; j++ {
				ws.Merge(map[string]string{fmt.Sprintf("f%d.py", i): "y", "a.py": "z"})
			}
		}(i)
		go func() {
			defer wg.Done()
			for j := 0; j < 500; j++ {
				ws.Set("a.py", "w")
				_, _ = ws.Get("a.py")
			}
		}()
		go func() {
			defer wg.Done()
			for j := 0; j < 500; j++ {
				for range ws.Snapshot() {
				}
			}
		}()
	}
	wg.Wait()
	if _, ok := ws.Get("a.py"); !ok {
		t.Fatal("a.py missing")
	}
}

func TestFileRead_RespectsDenyRules(t *testing.T) {
	deps := ToolDeps{
		Workspace: &Workspace{Files: map[string]string{"test_hidden.py": "secret", "app.py": "ok"}},
		Deny:      []DenyRule{{Tool: "FileRead", PathPattern: "test_hidden.py", Reason: "hidden test file is protected"}},
	}
	out, isErr, err := fileReadTool{}.Execute(context.Background(), map[string]any{"path": "test_hidden.py"}, deps)
	if err != nil || !isErr || strings.Contains(out, "secret") {
		t.Fatalf("hidden file readable: out=%q isErr=%v err=%v", out, isErr, err)
	}
	out, isErr, _ = fileReadTool{}.Execute(context.Background(), map[string]any{"path": "app.py"}, deps)
	if isErr || out != "ok" {
		t.Fatalf("allowed file: out=%q isErr=%v", out, isErr)
	}
}

func TestFileRead_ReadsSelectedLines(t *testing.T) {
	deps := ToolDeps{
		Workspace: &Workspace{
			Files: map[string]string{
				"app.py": "line 1\nline 2\nline 3\nline 4\nline 5",
			},
		},
	}

	out, isErr, err := fileReadTool{}.Execute(
		context.Background(),
		map[string]any{
			"path":       "app.py",
			"start_line": float64(2),
			"line_count": float64(2),
		},
		deps,
	)
	if err != nil || isErr {
		t.Fatalf("partial read failed: out=%q isErr=%v err=%v", out, isErr, err)
	}
	if out != "line 2\nline 3" {
		t.Fatalf("partial read = %q, want %q", out, "line 2\nline 3")
	}
}

func TestFileRead_PreservesFullFileRead(t *testing.T) {
	const content = "line 1\nline 2\nline 3"
	deps := ToolDeps{
		Workspace: &Workspace{
			Files: map[string]string{"app.py": content},
		},
	}

	out, isErr, err := fileReadTool{}.Execute(
		context.Background(),
		map[string]any{"path": "app.py"},
		deps,
	)
	if err != nil || isErr {
		t.Fatalf("full read failed: out=%q isErr=%v err=%v", out, isErr, err)
	}
	if out != content {
		t.Fatalf("full read = %q, want %q", out, content)
	}
}

func TestFileEdit_ReplacesExactBlock(t *testing.T) {
	workspace := &Workspace{
		Files: map[string]string{
			"app.py": "before\nvalue = 1\nafter\n",
		},
	}
	deps := ToolDeps{Workspace: workspace}

	out, isErr, err := fileEditTool{}.Execute(
		context.Background(),
		map[string]any{
			"path":     "app.py",
			"old_text": "value = 1",
			"new_text": "value = 2",
		},
		deps,
	)
	if err != nil || isErr {
		t.Fatalf(
			"targeted edit failed: out=%q isErr=%v err=%v",
			out,
			isErr,
			err,
		)
	}

	got, _ := workspace.Get("app.py")
	want := "before\nvalue = 2\nafter\n"
	if got != want {
		t.Fatalf("edited file=%q want %q", got, want)
	}
}

func TestFileEdit_RejectsRepeatedOldText(t *testing.T) {
	workspace := &Workspace{
		Files: map[string]string{
			"app.py": "value = 1\nvalue = 1\n",
		},
	}
	deps := ToolDeps{Workspace: workspace}

	_, isErr, err := fileEditTool{}.Execute(
		context.Background(),
		map[string]any{
			"path":     "app.py",
			"old_text": "value = 1",
			"new_text": "value = 2",
		},
		deps,
	)
	if err != nil {
		t.Fatalf("unexpected execution error: %v", err)
	}
	if !isErr {
		t.Fatal("expected repeated old_text to be rejected")
	}
}

func TestFileRead_RejectsInvalidRange(t *testing.T) {
	deps := ToolDeps{
		Workspace: &Workspace{
			Files: map[string]string{"app.py": "line 1\nline 2"},
		},
	}

	out, isErr, err := fileReadTool{}.Execute(
		context.Background(),
		map[string]any{
			"path":       "app.py",
			"start_line": float64(0),
		},
		deps,
	)
	if err != nil || !isErr || !strings.Contains(out, "positive integer") {
		t.Fatalf(
			"invalid range: out=%q isErr=%v err=%v",
			out,
			isErr,
			err,
		)
	}
}

func TestDefaultRegistry_NoHostShell(t *testing.T) {
	if _, ok := NewDefaultRegistry()["RunCommand"]; ok {
		t.Fatal("RunCommand must be opt-in: it executes on the backend host")
	}
	if _, ok := NewDefaultRegistry().WithHostCommands()["RunCommand"]; !ok {
		t.Fatal("WithHostCommands should add RunCommand")
	}
}
