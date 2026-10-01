package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolvePaths_FindsRootFromSubdir(t *testing.T) {
	root := t.TempDir()
	for _, d := range []string{"migrations", "backend/internal/x"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(filepath.Join(root, "backend", "internal", "x"))
	t.Setenv("CODRITIUM_ROOT", "")
	t.Setenv("SANDBOX_PYTHON", "")

	p, err := ResolvePaths()
	if err != nil {
		t.Fatal(err)
	}
	want, _ := filepath.EvalSymlinks(root)
	got, _ := filepath.EvalSymlinks(p.Root)
	if got != want {
		t.Fatalf("root=%q want %q", got, want)
	}
	if p.Migrations != filepath.Join(p.Root, "migrations") || p.ProblemsSeed != filepath.Join(p.Root, "seed", "problems") {
		t.Fatalf("paths=%+v", p)
	}
	if !strings.HasPrefix(p.SandboxPython, filepath.Join(p.Root, "sandbox", ".venv")) {
		t.Fatalf("python=%q", p.SandboxPython)
	}
}

func TestResolvePaths_EnvOverrides(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CODRITIUM_ROOT", root)
	t.Setenv("SANDBOX_PYTHON", "/opt/py/bin/python3")
	p, err := ResolvePaths()
	if err != nil {
		t.Fatal(err)
	}
	if p.SandboxPython != "/opt/py/bin/python3" {
		t.Fatalf("python=%q", p.SandboxPython)
	}
	if p.SandboxScript != filepath.Join(p.Root, "sandbox", "run_pytest.py") {
		t.Fatalf("script=%q", p.SandboxScript)
	}
}
