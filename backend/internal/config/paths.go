package config

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
)

// Paths locates the repo's non-Go assets. The server used to reach them
// with "../migrations"-style paths, which only worked when started from
// backend/. Now the repo root is found once (CODRITIUM_ROOT, or by walking
// up from the working directory) and everything hangs off it.
type Paths struct {
	Root          string
	Migrations    string
	Seed          string
	ProblemsSeed  string
	SandboxScript string
	// SandboxPython is the interpreter for the E2B wrapper: SANDBOX_PYTHON,
	// or sandbox/.venv's python for this OS (bin/python vs Scripts\python.exe).
	SandboxPython string
}

// ResolvePaths finds the repo root and derives every asset path from it.
func ResolvePaths() (Paths, error) {
	root := os.Getenv("CODRITIUM_ROOT")
	if root == "" {
		var err error
		if root, err = findRoot(); err != nil {
			return Paths{}, err
		}
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return Paths{}, err
	}
	p := Paths{
		Root:          root,
		Migrations:    filepath.Join(root, "migrations"),
		Seed:          filepath.Join(root, "seed"),
		ProblemsSeed:  filepath.Join(root, "seed", "problems"),
		SandboxScript: filepath.Join(root, "sandbox", "run_pytest.py"),
		SandboxPython: os.Getenv("SANDBOX_PYTHON"),
	}
	if p.SandboxPython == "" {
		p.SandboxPython = venvPython(filepath.Join(root, "sandbox", ".venv"))
	}
	return p, nil
}

func venvPython(venv string) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(venv, "Scripts", "python.exe")
	}
	return filepath.Join(venv, "bin", "python")
}

// findRoot walks up from the working directory to the first directory that
// holds both migrations/ and backend/.
func findRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if isDir(filepath.Join(dir, "migrations")) && isDir(filepath.Join(dir, "backend")) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("repo root not found: run inside the repo or set CODRITIUM_ROOT")
		}
		dir = parent
	}
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
