// Package archtest enforces the backend's module boundaries and size
// budget, so the structure doesn't erode back into one big package.
package archtest

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const module = "codritium/backend/"

// MaxFileLines is the budget for one non-test Go file. Split by concern
// (handler / store / model) before a file grows past it.
const MaxFileLines = 600

type pkg struct {
	ImportPath string
	Dir        string
	GoFiles    []string
	Imports    []string
}

func listPackages(t *testing.T) []pkg {
	t.Helper()
	cmd := exec.Command("go", "list", "-json", "./...")
	cmd.Dir = "../.."
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Skipf("go tool unavailable: %v", err)
	}
	var pkgs []pkg
	dec := json.NewDecoder(out)
	for {
		var p pkg
		if err := dec.Decode(&p); err == io.EOF {
			break
		} else if err != nil {
			t.Fatal(err)
		}
		pkgs = append(pkgs, p)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatal(err)
	}
	return pkgs
}

func rel(path string) string { return strings.TrimPrefix(path, module) }

// Rules: each is (importer prefix, forbidden import prefix, why).
var rules = []struct{ from, to, why string }{
	{"internal/platform/httpx", "internal/", "httpx is shared plumbing; it must not depend on features"},
	{"internal/", "internal/app", "only cmd/server wires the app"},
	{"internal/", "cmd/", "packages never import commands"},
	{"internal/llm", "internal/agent", "the agent runtime must not depend on its HTTP layer"},
	{"internal/problems", "internal/submissions", "problems is a leaf domain"},
	{"internal/sessions", "internal/submissions", "sessions is a leaf domain"},
}

// violation returns why from may not import to, or "".
func violation(from, to string) string {
	if from == to || strings.HasPrefix(from, "internal/archtest") {
		return ""
	}
	for _, r := range rules {
		if !strings.HasPrefix(from, r.from) || !strings.HasPrefix(to, r.to) {
			continue
		}
		if r.to == "internal/" && strings.HasPrefix(to, "internal/platform/") {
			continue // platform packages may use each other
		}
		if r.from == "internal/" && strings.HasPrefix(from, r.to) {
			continue // e.g. internal/app may import its own subpackages
		}
		return r.why
	}
	return ""
}

func TestViolationRules(t *testing.T) {
	cases := []struct {
		from, to string
		bad      bool
	}{
		{"internal/problems", "internal/app", true},
		{"internal/platform/httpx", "internal/auth", true},
		{"internal/platform/httpx", "internal/platform/testutil", false},
		{"internal/llm", "internal/agent", true},
		{"internal/agent", "internal/llm", false},
		{"internal/sessions", "internal/submissions", true},
		{"internal/submissions", "internal/sessions", false},
		{"internal/app", "internal/problems", false},
		{"internal/forum", "cmd/server", true},
	}
	for _, c := range cases {
		if got := violation(c.from, c.to) != ""; got != c.bad {
			t.Errorf("%s -> %s: violation=%v want %v", c.from, c.to, got, c.bad)
		}
	}
}

func TestModuleBoundaries(t *testing.T) {
	for _, p := range listPackages(t) {
		for _, imp := range p.Imports {
			if !strings.HasPrefix(imp, module) {
				continue
			}
			if why := violation(rel(p.ImportPath), rel(imp)); why != "" {
				t.Errorf("%s imports %s: %s", rel(p.ImportPath), rel(imp), why)
			}
		}
	}
}

func TestFileSizeBudget(t *testing.T) {
	for _, p := range listPackages(t) {
		for _, f := range p.GoFiles {
			path := filepath.Join(p.Dir, f)
			n := countLines(t, path)
			if n > MaxFileLines {
				t.Errorf("%s/%s has %d lines (budget %d): split it by concern", rel(p.ImportPath), f, n, MaxFileLines)
			}
		}
	}
}

func countLines(t *testing.T, path string) int {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	n := 0
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		n++
	}
	return n
}
