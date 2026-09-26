package e2b

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	apiBase     = "https://api.e2b.dev"
	httpTimeout = 30 * time.Second
)

type Client struct {
	apiKey string
	http   *http.Client
}

func New(apiKey string) *Client {
	return &Client{
		apiKey: apiKey,
		http:   &http.Client{Timeout: httpTimeout},
	}
}

// SandboxInfo is the metadata for a created sandbox.
type SandboxInfo struct {
	SandboxID    string `json:"sandboxID"`
	TemplateID   string `json:"templateID"`
	ClientID     string `json:"clientID"`
	AliasID      string `json:"aliasID,omitempty"`
	EnvVars      any    `json:"envVars,omitempty"`
}

// CreateRequest is what we POST to /sandboxes.
type CreateRequest struct {
	TemplateID string            `json:"templateID"`
	EnvVars    map[string]string `json:"envVars,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
	Timeout    int               `json:"timeout,omitempty"` // seconds
}

// Create starts a new sandbox using the specified template (e.g. "base", "code-interpreter-v1").
func (c *Client) Create(ctx context.Context, req CreateRequest) (*SandboxInfo, error) {
	body, _ := json.Marshal(req)
	r, err := http.NewRequestWithContext(ctx, "POST", apiBase+"/sandboxes", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	r.Header.Set("X-API-Key", c.apiKey)
	r.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(r)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("e2b create sandbox failed: %d %s", resp.StatusCode, string(b))
	}

	var info SandboxInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, err
	}
	return &info, nil
}

// Kill terminates a sandbox.
func (c *Client) Kill(ctx context.Context, sandboxID string) error {
	r, _ := http.NewRequestWithContext(ctx, "DELETE", apiBase+"/sandboxes/"+sandboxID, nil)
	r.Header.Set("X-API-Key", c.apiKey)
	resp, err := c.http.Do(r)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 && resp.StatusCode != http.StatusNotFound {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("e2b kill failed: %d %s", resp.StatusCode, string(b))
	}
	return nil
}

// Ping verifies the API key by listing templates (which requires auth).
// Returns the count of available templates as a sanity signal.
func (c *Client) Ping(ctx context.Context) (int, error) {
	r, _ := http.NewRequestWithContext(ctx, "GET", apiBase+"/templates", nil)
	r.Header.Set("X-API-Key", c.apiKey)
	resp, err := c.http.Do(r)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("e2b ping failed: %d %s", resp.StatusCode, string(b))
	}

	var templates []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&templates); err != nil {
		// Some E2B API revisions wrap in {"templates": [...]}; try that too.
		return 0, fmt.Errorf("decode templates list: %w", err)
	}
	return len(templates), nil
}
