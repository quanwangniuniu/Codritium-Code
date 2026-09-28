package e2b

import (
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
