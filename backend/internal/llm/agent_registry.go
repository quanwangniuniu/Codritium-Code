package llm

import (
	"context"
	"sync"

	"github.com/google/uuid"
)

// AgentFactory builds a fresh Agent given a session id and challenge slug.
// Returned Agent must be ready to receive RunTurn calls — its Workspace,
// system prompt, deny rules, and stream client should already be set.
type AgentFactory func(ctx context.Context, sessionID uuid.UUID, challengeSlug string) (*Agent, error)

// AgentRegistry keeps one Agent per active session in process memory.
// Server restart drops everything (acceptable for v0.8; v1.0 will
// rehydrate from session_messages — decision_log D2 forward).
type AgentRegistry struct {
	mu      sync.Mutex
	agents  map[uuid.UUID]*Agent
	factory AgentFactory
}

func NewAgentRegistry(factory AgentFactory) *AgentRegistry {
	return &AgentRegistry{
		agents:  make(map[uuid.UUID]*Agent),
		factory: factory,
	}
}

// GetOrCreate returns the existing Agent for sessionID, or constructs a
// fresh one via the factory and caches it.
func (r *AgentRegistry) GetOrCreate(ctx context.Context, sessionID uuid.UUID, challengeSlug string) (*Agent, error) {
	r.mu.Lock()
	if a, ok := r.agents[sessionID]; ok {
		r.mu.Unlock()
		return a, nil
	}
	r.mu.Unlock()

	a, err := r.factory(ctx, sessionID, challengeSlug)
	if err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	// re-check in case another goroutine raced us
	if existing, ok := r.agents[sessionID]; ok {
		return existing, nil
	}
	r.agents[sessionID] = a
	return a, nil
}

// Drop forgets the Agent for sessionID (e.g. when the session is
// submitted). Safe to call when no Agent is cached.
func (r *AgentRegistry) Drop(sessionID uuid.UUID) {
	r.mu.Lock()
	delete(r.agents, sessionID)
	r.mu.Unlock()
}
