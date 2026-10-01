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
