package events

import (
	"context"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// These tests require a running Postgres with the v0.8 migration applied.
// They set up and tear down their own candidate_sessions rows so they do
// not leak state across runs.

func setupPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://codritium:codritium@localhost:5434/codritium?sslmode=disable"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Skipf("no postgres reachable at %s: %v", dsn, err)
	}
	if err := pool.Ping(ctx); err != nil {
		t.Skipf("postgres ping failed: %v", err)
	}
	return pool
}

// makeSession creates a candidate_sessions row and registers cleanup.
func makeSession(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := pool.QueryRow(context.Background(), `
		INSERT INTO candidate_sessions (candidate_id, challenge_id, difficulty, candidate_role)
		VALUES ($1, $2, 'medium', 'senior_backend')
		RETURNING session_id`,
		"test-"+uuid.NewString(),
		"22-build-rate-limiter-middleware",
	).Scan(&id)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	t.Cleanup(func() {
		// cascade-deletes session_events / session_messages too
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM candidate_sessions WHERE session_id = $1`, id)
	})
	return id
}

func TestKnownKinds_Has16Kinds(t *testing.T) {
	// Spec sets the closed set; defend against accidental kind drift.
	const want = 16
	if got := len(KnownKinds); got != want {
		t.Fatalf("KnownKinds len = %d, want %d", got, want)
	}
	for _, k := range []string{"ai_output_read", "candidate_reverted_edit"} {
		if _, ok := KnownKinds[k]; !ok {
			t.Fatalf("R5 event missing from KnownKinds: %q", k)
		}
	}
}

func TestR5Events_RoundTripThroughStore(t *testing.T) {
	pool := setupPool(t)
	store := NewStore(pool)
	sid := makeSession(t, pool)

	if _, err := store.Append(context.Background(), sid, AIOutputRead{
		MessageID:          "msg-3-2",
		PauseDurationSec:   47,
		ScrollDepthPercent: 100,
		NextActionKind:     "new_prompt",
	}); err != nil {
		t.Fatalf("append ai_output_read: %v", err)
	}
	if _, err := store.Append(context.Background(), sid, CandidateRevertedEdit{
		OriginalToolUseID: "tu-3-1",
		RevertMethod:      "undo",
		FilesAffected:     []string{"limiter_store.py"},
		NextActionKind:    "new_prompt",
	}); err != nil {
		t.Fatalf("append candidate_reverted_edit: %v", err)
	}
}

func TestStore_AppendMonotonicSeq(t *testing.T) {
	pool := setupPool(t)
	store := NewStore(pool)
	sid := makeSession(t, pool)

	got := make([]int64, 0, 5)
	for i := 0; i < 5; i++ {
		env, err := store.Append(context.Background(), sid, SessionStarted{
			ChallengeID: "22-build-rate-limiter-middleware",
			Difficulty:  "medium",
		})
		if err != nil {
			t.Fatalf("append #%d: %v", i, err)
		}
		got = append(got, env.Seq)
	}
	want := []int64{1, 2, 3, 4, 5}
	for i, w := range want {
		if got[i] != w {
			t.Fatalf("seq[%d] = %d, want %d (full: %v)", i, got[i], w, got)
		}
	}
}

func TestStore_RejectsUnknownKind(t *testing.T) {
	pool := setupPool(t)
	store := NewStore(pool)
	sid := makeSession(t, pool)

	_, err := store.Append(context.Background(), sid, fakeEvent{})
	if err == nil {
		t.Fatal("expected error on unknown kind, got nil")
	}
}

type fakeEvent struct{}

func (fakeEvent) Kind() string  { return "bogus_kind_does_not_exist" }
func (f fakeEvent) Payload() any { return f }

func TestStore_BootstrapFromExistingMax(t *testing.T) {
	pool := setupPool(t)
	sid := makeSession(t, pool)

	// Insert two events bypassing the Store, to simulate "process restarted
	// mid-session". The next Store run must pick seq=3.
	for _, seq := range []int64{1, 2} {
		_, err := pool.Exec(context.Background(), `
			INSERT INTO session_events (session_id, seq, kind, payload)
			VALUES ($1, $2, 'session_started', '{}'::jsonb)`,
			sid, seq)
		if err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	store := NewStore(pool)
	env, err := store.Append(context.Background(), sid, SessionStarted{
		ChallengeID: "x",
		Difficulty:  "medium",
	})
	if err != nil {
		t.Fatalf("append: %v", err)
	}
	if env.Seq != 3 {
		t.Fatalf("bootstrap seq = %d, want 3", env.Seq)
	}
}

func TestStore_AppendMessageSharesSeqCounter(t *testing.T) {
	pool := setupPool(t)
	sid := makeSession(t, pool)
	ctx := context.Background()

	store := NewStore(pool)
	if _, err := store.Append(ctx, sid, SessionStarted{ChallengeID: "x", Difficulty: "medium"}); err != nil {
		t.Fatalf("append event: %v", err)
	}
	if err := store.AppendMessage(ctx, sid, "user", []byte(`[{"type":"text","text":"hi"}]`)); err != nil {
		t.Fatalf("append message: %v", err)
	}

	var role, text string
	var seq int64
	if err := pool.QueryRow(ctx, `
		SELECT role, content->0->>'text', seq FROM session_messages WHERE session_id = $1`,
		sid).Scan(&role, &text, &seq); err != nil {
		t.Fatalf("read message: %v", err)
	}
	if role != "user" || text != "hi" || seq != 2 {
		t.Fatalf("message = (%q, %q, seq %d), want (user, hi, seq 2)", role, text, seq)
	}

	// A fresh Store (process restart) must bootstrap past the transcript's
	// seq, not just session_events'.
	if err := NewStore(pool).AppendMessage(ctx, sid, "assistant", []byte(`[]`)); err != nil {
		t.Fatalf("append after restart: %v", err)
	}
	env, err := NewStore(pool).Append(ctx, sid, SessionStarted{ChallengeID: "x", Difficulty: "medium"})
	if err != nil {
		t.Fatalf("append event after restart: %v", err)
	}
	if env.Seq != 4 {
		t.Fatalf("seq after restart = %d, want 4", env.Seq)
	}
}

func TestStore_ConcurrentAppendUniqueSeq(t *testing.T) {
	pool := setupPool(t)
	store := NewStore(pool)
	sid := makeSession(t, pool)

	const n = 50
	var wg sync.WaitGroup
	seen := sync.Map{}
	var errCount atomic.Int32

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			env, err := store.Append(context.Background(), sid, SessionStarted{
				ChallengeID: "x",
				Difficulty:  "medium",
			})
			if err != nil {
				errCount.Add(1)
				return
			}
			if _, dup := seen.LoadOrStore(env.Seq, true); dup {
				errCount.Add(1)
			}
		}()
	}
	wg.Wait()
	if errCount.Load() > 0 {
		t.Fatalf("errors during concurrent append: %d", errCount.Load())
	}

	// 1..n must be present exactly once
	for i := int64(1); i <= n; i++ {
		if _, ok := seen.Load(i); !ok {
			t.Fatalf("missing seq %d", i)
		}
	}
}

func TestStore_SubscribeReceivesEnvelopes(t *testing.T) {
	pool := setupPool(t)
	store := NewStore(pool)
	sid := makeSession(t, pool)

	ch := make(chan Envelope, 4)
	cancel := store.Subscribe(sid, ch)
	defer cancel()

	for i := 0; i < 3; i++ {
		if _, err := store.Append(context.Background(), sid, SessionStarted{
			ChallengeID: "x",
			Difficulty:  "medium",
		}); err != nil {
			t.Fatalf("append: %v", err)
		}
	}

	timeout := time.After(2 * time.Second)
	got := []int64{}
	for len(got) < 3 {
		select {
		case env := <-ch:
			got = append(got, env.Seq)
			if env.Kind != "session_started" {
				t.Fatalf("env.Kind = %q", env.Kind)
			}
		case <-timeout:
			t.Fatalf("only received %d envelopes: %v", len(got), got)
		}
	}
	for i, want := range []int64{1, 2, 3} {
		if got[i] != want {
			t.Fatalf("got[%d]=%d want %d", i, got[i], want)
		}
	}
}

func TestStore_SubscribeIsolatedPerSession(t *testing.T) {
	pool := setupPool(t)
	store := NewStore(pool)
	sidA := makeSession(t, pool)
	sidB := makeSession(t, pool)

	chA := make(chan Envelope, 4)
	store.Subscribe(sidA, chA)

	// Append on sidB; chA must not see it.
	if _, err := store.Append(context.Background(), sidB, SessionStarted{
		ChallengeID: "x",
		Difficulty:  "medium",
	}); err != nil {
		t.Fatalf("append B: %v", err)
	}

	select {
	case env := <-chA:
		t.Fatalf("chA received env for sidB: %+v", env)
	case <-time.After(200 * time.Millisecond):
		// expected: nothing
	}
}

func TestStore_SubscribeDropsOnSlowConsumer(t *testing.T) {
	pool := setupPool(t)
	store := NewStore(pool)
	sid := makeSession(t, pool)

	// Channel capacity 1 — second envelope will be dropped, not block.
	ch := make(chan Envelope, 1)
	store.Subscribe(sid, ch)

	// Two appends back to back; the second one must not block Append.
	for i := 0; i < 2; i++ {
		_, err := store.Append(context.Background(), sid, SessionStarted{
			ChallengeID: "x",
			Difficulty:  "medium",
		})
		if err != nil {
			t.Fatalf("append #%d: %v", i, err)
		}
	}

	// One envelope received, the other dropped.
	select {
	case <-ch:
		// fine
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected at least one envelope buffered")
	}
}
