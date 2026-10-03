package llm

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/google/uuid"
)

// SessionTmpFS is the per-session scratch directory the agent tools
// Grep / Glob / RunCommand operate against. It mirrors the candidate
// workspace under /tmp/codritium/<session_id>/ so tools can spawn external
// processes without touching the host filesystem. The hidden test file is
// never materialized; callers pass its path in skipPaths.
type SessionTmpFS struct {
	sessionID uuid.UUID
	dir       string
	skipPaths map[string]struct{}

	mu      sync.Mutex // serializes Sync against Cleanup
	cleaned bool       // set by Cleanup; Sync refuses to recreate the dir
}

// NewSessionTmpFS creates the backing directory. Callers are expected to
// invoke Cleanup when the session ends; absent that the directory sits in
// /tmp until the host's tmpwatch policy reaps it.
func NewSessionTmpFS(sessionID uuid.UUID, skipPaths []string) (*SessionTmpFS, error) {
	dir := filepath.Join(os.TempDir(), "codritium", sessionID.String())
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("session tmpfs mkdir: %w", err)
	}
	skip := make(map[string]struct{}, len(skipPaths))
	for _, p := range skipPaths {
		if p == "" {
			continue
		}
		skip[filepath.Clean(p)] = struct{}{}
	}
	return &SessionTmpFS{sessionID: sessionID, dir: dir, skipPaths: skip}, nil
}

// Dir returns the absolute scratch directory the tools chdir into.
func (s *SessionTmpFS) Dir() string {
	if s == nil {
		return ""
	}
	return s.dir
}

// Sync writes the workspace state to disk, skipping any path in the deny
// set and any path containing a parent-traversal segment. Files whose
// content matches what is already on disk are left untouched so a tool can
// observe stable mtimes.
func (s *SessionTmpFS) Sync(files map[string]string) error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cleaned {
		return fmt.Errorf("session tmpfs %s already cleaned up", s.sessionID)
	}
	for path, content := range files {
		clean := filepath.Clean(path)
		if _, skip := s.skipPaths[clean]; skip {
			continue
		}
		if strings.Contains(clean, "..") || filepath.IsAbs(clean) {
			continue
		}
		full := filepath.Join(s.dir, clean)
		if err := os.MkdirAll(filepath.Dir(full), 0o700); err != nil {
			return err
		}
		if existing, err := os.ReadFile(full); err == nil && string(existing) == content {
			continue
		}
		if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
			return err
		}
	}
	return nil
}

// Cleanup removes the session directory and everything beneath it.
func (s *SessionTmpFS) Cleanup() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleaned = true
	return os.RemoveAll(s.dir)
}
