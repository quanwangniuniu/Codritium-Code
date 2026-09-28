package events

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store appends events to session_events with a per-session monotonic seq.
//
// The seq counter is owned by the application, not the database. On first
// access for a given session_id the counter bootstraps from
// SELECT COALESCE(MAX(seq), 0); thereafter sub-millisecond Append calls are
// lock-free (atomic.AddInt64). See decision_log.md D6: this counter is also
// used by the transcript table (session_messages) so audit replay is
// totally ordered across both tables.
//
// Subscribe() lets callers (e.g. the SSE chat handler) receive every
// envelope as it is persisted. Subscribers are fanned out best-effort —
// slow consumers drop messages rather than block Append.
type Store struct {
	pool *pgxpool.Pool

	// per-session seq counters. value type: *sessionSeq.
	seq sync.Map

	// per-session subscriber slices. value type: []chan<- Envelope.
	// Protected by subMu when mutated.
	subMu sync.Mutex
	subs  sync.Map
}

type sessionSeq struct {
	once sync.Once
	val  int64 // accessed via sync/atomic
}

// NewStore returns a Store backed by the given pool. Callers are expected to
// keep the Store alive for the process lifetime — its in-memory seq cache
// must outlast every chat goroutine in a given session.
func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// NextSeq returns the next seq number for a session and is exposed so that
// the transcript writer (session_messages) can share the same counter.
// Most callers should not call NextSeq directly — use Append.
func (s *Store) NextSeq(ctx context.Context, sessionID uuid.UUID) (int64, error) {
	raw, _ := s.seq.LoadOrStore(sessionID, &sessionSeq{})
	ss := raw.(*sessionSeq)
	var bootstrapErr error
	ss.once.Do(func() {
		var maxSeq sql.NullInt64
		err := s.pool.QueryRow(ctx,
			`SELECT COALESCE(MAX(seq), 0) FROM session_events WHERE session_id = $1`,
			sessionID,
		).Scan(&maxSeq)
		if err != nil {
			bootstrapErr = fmt.Errorf("bootstrap seq for %s: %w", sessionID, err)
			return
		}
		atomic.StoreInt64(&ss.val, maxSeq.Int64)
	})
	if bootstrapErr != nil {
		// Clear the entry so a retry can re-bootstrap; sync.Once would
		// otherwise permanently silence retries.
		s.seq.Delete(sessionID)
		return 0, bootstrapErr
	}
	return atomic.AddInt64(&ss.val, 1), nil
}

// Append persists one event and fans it out to subscribers. The returned
// envelope carries the assigned seq and the server-side emitted_at.
func (s *Store) Append(ctx context.Context, sessionID uuid.UUID, ev Event) (Envelope, error) {
	if _, ok := KnownKinds[ev.Kind()]; !ok {
		return Envelope{}, fmt.Errorf("events: unknown kind %q", ev.Kind())
	}
	seq, err := s.NextSeq(ctx, sessionID)
	if err != nil {
		return Envelope{}, err
	}
	payload := ev.Payload()
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return Envelope{}, fmt.Errorf("marshal payload: %w", err)
	}

	var emittedAt time.Time
	err = s.pool.QueryRow(ctx, `
		INSERT INTO session_events (session_id, seq, kind, payload)
		VALUES ($1, $2, $3, $4::jsonb)
		RETURNING emitted_at`,
		sessionID, seq, ev.Kind(), string(payloadJSON),
	).Scan(&emittedAt)
	if err != nil {
		return Envelope{}, fmt.Errorf("insert event: %w", err)
	}

	env := Envelope{
		SessionID: sessionID,
		Seq:       seq,
		Kind:      ev.Kind(),
		EmittedAt: emittedAt.UTC().Format(time.RFC3339Nano),
		Payload:   payload,
	}
	s.fanout(sessionID, env)
	return env, nil
}

// Subscribe registers a channel to receive every Envelope persisted for the
// given session. The returned cancel removes the subscription. The channel
// is never closed by the Store — callers own its lifecycle. If the channel
// is full when an envelope arrives, the envelope is dropped (see contract
// on the type doc).
func (s *Store) Subscribe(sessionID uuid.UUID, ch chan<- Envelope) (cancel func()) {
	s.subMu.Lock()
	defer s.subMu.Unlock()
	raw, _ := s.subs.LoadOrStore(sessionID, []chan<- Envelope{})
	list := raw.([]chan<- Envelope)
	list = append(list, ch)
	s.subs.Store(sessionID, list)
	return func() {
		s.subMu.Lock()
		defer s.subMu.Unlock()
		raw, ok := s.subs.Load(sessionID)
		if !ok {
			return
		}
		list := raw.([]chan<- Envelope)
		for i, c := range list {
			if c == ch {
				list = append(list[:i], list[i+1:]...)
				break
			}
		}
		if len(list) == 0 {
			s.subs.Delete(sessionID)
		} else {
			s.subs.Store(sessionID, list)
		}
	}
}

func (s *Store) fanout(sessionID uuid.UUID, env Envelope) {
	raw, ok := s.subs.Load(sessionID)
	if !ok {
		return
	}
	list := raw.([]chan<- Envelope)
	for _, ch := range list {
		select {
		case ch <- env:
		default:
			// slow consumer — drop
		}
	}
}
