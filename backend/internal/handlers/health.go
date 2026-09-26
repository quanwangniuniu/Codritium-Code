package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"codritium/backend/internal/anthropic"
	"codritium/backend/internal/e2b"
)

type HealthDeps struct {
	Anthropic *anthropic.Client
	E2B       *e2b.Client
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// Health is a liveness probe — no external calls.
func Health() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"status": "ok",
			"time":   time.Now().UTC().Format(time.RFC3339),
		})
	}
}

// AnthropicPing calls Sonnet with a tiny prompt and returns the reply.
func AnthropicPing(deps HealthDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()

		reply, err := deps.Anthropic.Ping(ctx)
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]any{"status": "error", "error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "reply": reply})
	}
}

// E2BPing lists templates as a sanity check that the API key works.
func E2BPing(deps HealthDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()

		count, err := deps.E2B.Ping(ctx)
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]any{"status": "error", "error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "templates_count": count})
	}
}
