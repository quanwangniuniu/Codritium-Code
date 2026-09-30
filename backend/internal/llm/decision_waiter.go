package llm

import (
	"context"
	"errors"
	"sync"

	"github.com/google/uuid"
)

// DecisionKey scopes a pending decision to one session so tool_use ids from
// different sessions (or providers that reuse ids) can never collide, and a
// decision can only reach the session it was proposed in.
func DecisionKey(sessionID uuid.UUID, toolUseID string) string {
	return sessionID.String() + "/" + toolUseID
}

// Decision is the candidate's verdict on one ToolUseProposed.
type Decision struct {
	// Kind is one of "approve" | "reject" | "modify".
	Kind string
	// ModifiedInput is the candidate's edited tool input (modify only).
	// JSON-encoded; the chat loop hands this straight to executeTool.
	ModifiedInput string
	// Reason is the candidate's explanation for a reject. Optional.
	Reason string
	// Comment is the candidate's optional remark when approving.
	Comment string
}

// IsValid reports whether Kind is one of the three accepted strings.
// Used by the HTTP handler at the boundary; the chat loop trusts what it
// gets back from Wait.
func (d Decision) IsValid() bool {
	switch d.Kind {
	case "approve", "reject", "modify":
		return true
	}
	return false
}

// DecisionWaiter is the rendezvous between the chat goroutine (Wait) and
// the HTTP handler that receives the candidate's POST /api/decision
// (Notify). One pending entry per outstanding key; callers build keys
// with DecisionKey.
//
// v0.8 is single-machine, so the pending map is in-process memory. v1.0
// will swap this for Redis pub/sub (see HANDOFF §5).
type DecisionWaiter struct {
	mu      sync.Mutex
	pending map[string]chan Decision
}

// NewDecisionWaiter returns a fresh waiter ready to accept Wait() calls.
func NewDecisionWaiter() *DecisionWaiter {
	return &DecisionWaiter{pending: make(map[string]chan Decision)}
}

// ErrDuplicate is returned when Wait is invoked twice for the same
// tool_use_id without a Notify in between. The chat loop should never
// do this — the id is unique per ToolUseProposed.
var ErrDuplicate = errors.New("decision_waiter: duplicate tool_use_id")

// Wait blocks until Notify is called for toolUseID, or ctx is cancelled.
// On context cancellation the entry is cleaned up so a late Notify is a
// no-op rather than leaking the channel.
func (w *DecisionWaiter) Wait(ctx context.Context, toolUseID string) (Decision, error) {
	ch := make(chan Decision, 1)

	w.mu.Lock()
	if _, exists := w.pending[toolUseID]; exists {
		w.mu.Unlock()
		return Decision{}, ErrDuplicate
	}
	w.pending[toolUseID] = ch
	w.mu.Unlock()

	defer w.clear(toolUseID)

	select {
	case d := <-ch:
		return d, nil
	case <-ctx.Done():
		return Decision{}, ctx.Err()
	}
}

// Notify delivers a decision to a pending Wait. Returns true on success;
// false when no Wait is pending for that id (already notified, never
// registered, or ctx already cancelled).
func (w *DecisionWaiter) Notify(toolUseID string, d Decision) bool {
	w.mu.Lock()
	ch, ok := w.pending[toolUseID]
	w.mu.Unlock()
	if !ok {
		return false
	}
	select {
	case ch <- d:
		return true
	default:
		// channel already buffered or closed — treat as already notified
		return false
	}
}

// Pending reports whether a Wait is currently outstanding. Useful for
// tests and for the HTTP handler to return 404 cleanly.
func (w *DecisionWaiter) Pending(toolUseID string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	_, ok := w.pending[toolUseID]
	return ok
}

func (w *DecisionWaiter) clear(toolUseID string) {
	w.mu.Lock()
	delete(w.pending, toolUseID)
	w.mu.Unlock()
}
