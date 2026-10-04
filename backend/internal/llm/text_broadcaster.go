package llm

import (
	"sync"

	"github.com/google/uuid"
)

// TextBroadcaster is the side channel agent chat uses to push streamed
// assistant text deltas to subscribers, bypassing the events.Store. This
// keeps spec §5 "events ≠ transcript" honest — text never enters
// session_events.payload — while still giving the SSE layer one place
// to fan-out per-session text to many viewers.
//
// agent chat calls Publish during stream draining; the SSE handler holds
// a Subscribe channel for the SSE message-typed event stream. A slow
// consumer drops messages rather than block Publish.
type TextBroadcaster struct {
	mu   sync.Mutex
	subs sync.Map // sessionID → []chan<- string
}

func NewTextBroadcaster() *TextBroadcaster {
	return &TextBroadcaster{}
}

func (b *TextBroadcaster) Subscribe(sessionID uuid.UUID, ch chan<- string) (cancel func()) {
	b.mu.Lock()
	defer b.mu.Unlock()
	raw, _ := b.subs.LoadOrStore(sessionID, []chan<- string{})
	list := raw.([]chan<- string)
	list = append(list, ch)
	b.subs.Store(sessionID, list)
	return func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		raw, ok := b.subs.Load(sessionID)
		if !ok {
			return
		}
		list := raw.([]chan<- string)
		for i, c := range list {
			if c == ch {
				list = append(list[:i], list[i+1:]...)
				break
			}
		}
		if len(list) == 0 {
			b.subs.Delete(sessionID)
		} else {
			b.subs.Store(sessionID, list)
		}
	}
}

func (b *TextBroadcaster) Publish(sessionID uuid.UUID, delta string) {
	raw, ok := b.subs.Load(sessionID)
	if !ok {
		return
	}
	list := raw.([]chan<- string)
	for _, ch := range list {
		select {
		case ch <- delta:
		default:
			// slow consumer — drop
		}
	}
}
