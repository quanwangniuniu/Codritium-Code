package llm

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"codritium/backend/internal/events"
)

// ─── helpers ──────────────────────────────────────────────────────────

func newTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://codritium:codritium@localhost:5434/codritium?sslmode=disable"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Skipf("no postgres at %s: %v", dsn, err)
	}
	if err := pool.Ping(ctx); err != nil {
		t.Skipf("postgres ping: %v", err)
	}
	return pool
}

func newTestSession(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := pool.QueryRow(context.Background(), `
		INSERT INTO candidate_sessions (candidate_id, challenge_id, difficulty)
		VALUES ($1, $2, 'medium')
		RETURNING session_id`,
		"chat-test-"+uuid.NewString(),
		"22-build-rate-limiter-middleware",
	).Scan(&id)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM candidate_sessions WHERE session_id = $1`, id)
	})
	return id
}

// scriptedStream produces a predetermined chunk sequence per StreamTurn
// call. The script field is a slice-of-slices: scripts[i] is the chunks
// for the i-th StreamTurn call.
type scriptedStream struct {
	mu      sync.Mutex
	scripts [][]NormalizedChunk
	calls   int
}

func (s *scriptedStream) StreamTurn(ctx context.Context, req TurnRequest) (StreamReader, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.calls >= len(s.scripts) {
		return nil, fmt.Errorf("scripted stream exhausted (call %d)", s.calls)
	}
	chunks := s.scripts[s.calls]
	s.calls++
	return &scriptedReader{chunks: chunks}, nil
}

type scriptedReader struct {
	chunks []NormalizedChunk
	pos    int
}

func (r *scriptedReader) Next(ctx context.Context) (NormalizedChunk, bool, error) {
	if r.pos >= len(r.chunks) {
		return NormalizedChunk{}, false, nil
	}
	c := r.chunks[r.pos]
	r.pos++
	return c, true, nil
}

func (r *scriptedReader) Close() error { return nil }

// fakeSandbox stubs RunTests output without touching E2B.
type fakeSandbox struct {
	result SandboxResult
}

func (s *fakeSandbox) RunPytest(ctx context.Context, files map[string]string, testFile string) (SandboxResult, error) {
	return s.result, nil
}

func collectEvents(t *testing.T, pool *pgxpool.Pool, sid uuid.UUID) []string {
	t.Helper()
	rows, err := pool.Query(context.Background(),
		`SELECT kind FROM session_events WHERE session_id=$1 ORDER BY seq`, sid)
	if err != nil {
		t.Fatalf("query events: %v", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			t.Fatalf("scan: %v", err)
		}
		out = append(out, k)
	}
	return out
}

// ─── tests ────────────────────────────────────────────────────────────

func TestRunTurn_NoToolUseEndsNormal(t *testing.T) {
	pool := newTestPool(t)
	sid := newTestSession(t, pool)
	store := events.NewStore(pool)

	stream := &scriptedStream{
		scripts: [][]NormalizedChunk{
			{
				{Kind: "text_delta", Text: "Hello"},
				{Kind: "text_delta", Text: " world."},
				{Kind: "usage", InputTokens: 50, OutputTokens: 5},
				{Kind: "message_stop", StopReason: "end_turn"},
			},
		},
	}

	agent := &Agent{
		Stream: stream,
		Events: store,
		Waiter: NewDecisionWaiter(),
		Tools:  NewDefaultRegistry(),
		Workspace: &Workspace{Files: map[string]string{
			"rate_limiter.py": "starter",
		}},
	}

	res, err := agent.RunTurn(context.Background(), sid, 1, "hi")
	if err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if res.Reason != "normal" {
		t.Fatalf("reason=%q want normal", res.Reason)
	}
	if res.Iterations != 1 {
		t.Fatalf("iterations=%d want 1", res.Iterations)
	}
	if res.Usage.OutputTokens != 5 {
		t.Fatalf("usage.OutputTokens=%d want 5", res.Usage.OutputTokens)
	}

	kinds := collectEvents(t, pool, sid)
	wantHas := []string{"turn_completed"}
	for _, w := range wantHas {
		if !contains(kinds, w) {
			t.Fatalf("missing event %q in %v", w, kinds)
		}
	}
}

func TestRunTurn_ToolUseApproveRoundtrip(t *testing.T) {
	pool := newTestPool(t)
	sid := newTestSession(t, pool)
	store := events.NewStore(pool)
	waiter := NewDecisionWaiter()

	// script: turn 1 model proposes FileEdit; turn 2 (after tool_result)
	// model replies plain text, ends.
	stream := &scriptedStream{
		scripts: [][]NormalizedChunk{
			{
				{Kind: "text_delta", Text: "Here is an edit."},
				{Kind: "tool_use_start", ToolUseID: "tu-1", ToolUseName: "FileEdit"},
				{Kind: "tool_use_input_delta", ToolUseID: "tu-1", InputJSONDelta: `{"path":"rate_limiter.py","content":"# fixed"}`},
				{Kind: "tool_use_stop", ToolUseID: "tu-1"},
				{Kind: "usage", InputTokens: 100, OutputTokens: 30},
				{Kind: "message_stop", StopReason: "tool_use"},
			},
			{
				{Kind: "text_delta", Text: "Done."},
				{Kind: "usage", InputTokens: 130, OutputTokens: 3},
				{Kind: "message_stop", StopReason: "end_turn"},
			},
		},
	}

	agent := &Agent{
		Stream:    stream,
		Events:    store,
		Waiter:    waiter,
		Tools:     NewDefaultRegistry(),
		Workspace: &Workspace{Files: map[string]string{"rate_limiter.py": "starter"}},
	}

	// Notify approve once Wait has registered.
	go func() {
		for i := 0; i < 100; i++ {
			if waiter.Pending(DecisionKey(sid, "tu-1")) {
				waiter.Notify(DecisionKey(sid, "tu-1"), Decision{Kind: "approve"})
				return
			}
			time.Sleep(5 * time.Millisecond)
		}
		t.Errorf("Wait never registered tu-1")
	}()

	res, err := agent.RunTurn(context.Background(), sid, 1, "fix the bug")
	if err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if res.Reason != "normal" {
		t.Fatalf("reason=%q want normal", res.Reason)
	}
	if res.Iterations != 2 {
		t.Fatalf("iterations=%d want 2", res.Iterations)
	}

	// File should have been replaced
	if got := agent.Workspace.Files["rate_limiter.py"]; got != "# fixed" {
		t.Fatalf("workspace not updated; got %q", got)
	}

	// Event log: tool_use_proposed → candidate_approved → tool_result → turn_completed
	kinds := collectEvents(t, pool, sid)
	want := []string{"tool_use_proposed", "candidate_approved", "tool_result", "turn_completed"}
	for _, w := range want {
		if !contains(kinds, w) {
			t.Fatalf("missing event %q in %v", w, kinds)
		}
	}
}

func TestRunTurn_ToolUseRejected(t *testing.T) {
	pool := newTestPool(t)
	sid := newTestSession(t, pool)
	store := events.NewStore(pool)
	waiter := NewDecisionWaiter()

	stream := &scriptedStream{
		scripts: [][]NormalizedChunk{
			{
				{Kind: "tool_use_start", ToolUseID: "tu-2", ToolUseName: "FileEdit"},
				{Kind: "tool_use_input_delta", ToolUseID: "tu-2", InputJSONDelta: `{"path":"rate_limiter.py","content":"BAD"}`},
				{Kind: "tool_use_stop", ToolUseID: "tu-2"},
				{Kind: "message_stop", StopReason: "tool_use"},
			},
			{
				{Kind: "text_delta", Text: "OK, I'll try something else."},
				{Kind: "message_stop", StopReason: "end_turn"},
			},
		},
	}

	agent := &Agent{
		Stream:    stream,
		Events:    store,
		Waiter:    waiter,
		Tools:     NewDefaultRegistry(),
		Workspace: &Workspace{Files: map[string]string{"rate_limiter.py": "starter"}},
	}

	go func() {
		for i := 0; i < 100; i++ {
			if waiter.Pending(DecisionKey(sid, "tu-2")) {
				waiter.Notify(DecisionKey(sid, "tu-2"), Decision{Kind: "reject", Reason: "use sliding window instead"})
				return
			}
			time.Sleep(5 * time.Millisecond)
		}
	}()

	_, err := agent.RunTurn(context.Background(), sid, 1, "fix it")
	if err != nil {
		t.Fatalf("RunTurn: %v", err)
	}

	// File MUST NOT have changed
	if got := agent.Workspace.Files["rate_limiter.py"]; got != "starter" {
		t.Fatalf("workspace mutated on reject; got %q", got)
	}

	kinds := collectEvents(t, pool, sid)
	want := []string{"tool_use_proposed", "candidate_rejected", "turn_completed"}
	for _, w := range want {
		if !contains(kinds, w) {
			t.Fatalf("missing event %q in %v", w, kinds)
		}
	}
	// must NOT include tool_result for the rejected use (only message-level reject)
	for _, k := range kinds {
		if k == "tool_result" {
			t.Fatalf("rejected tool_use should not have emitted tool_result: %v", kinds)
		}
	}
}

func TestRunTurn_DenyRuleBlocksFileEdit(t *testing.T) {
	pool := newTestPool(t)
	sid := newTestSession(t, pool)
	store := events.NewStore(pool)
	waiter := NewDecisionWaiter()

	stream := &scriptedStream{
		scripts: [][]NormalizedChunk{
			{
				{Kind: "tool_use_start", ToolUseID: "tu-3", ToolUseName: "FileEdit"},
				{Kind: "tool_use_input_delta", ToolUseID: "tu-3", InputJSONDelta: `{"path":"test_answer.py","content":"def test_x(): pass"}`},
				{Kind: "tool_use_stop", ToolUseID: "tu-3"},
				{Kind: "message_stop", StopReason: "tool_use"},
			},
			{
				{Kind: "text_delta", Text: "Got it."},
				{Kind: "message_stop", StopReason: "end_turn"},
			},
		},
	}

	agent := &Agent{
		Stream: stream,
		Events: store,
		Waiter: waiter,
		Tools:  NewDefaultRegistry(),
		Workspace: &Workspace{Files: map[string]string{
			"rate_limiter.py": "x",
			"test_answer.py":  "original",
		}},
		Deny: []DenyRule{
			{Tool: "FileEdit", PathPattern: "test_answer.py", Reason: "visible test"},
		},
	}

	go func() {
		for i := 0; i < 100; i++ {
			if waiter.Pending(DecisionKey(sid, "tu-3")) {
				waiter.Notify(DecisionKey(sid, "tu-3"), Decision{Kind: "approve"})
				return
			}
			time.Sleep(5 * time.Millisecond)
		}
	}()

	_, err := agent.RunTurn(context.Background(), sid, 1, "alter the test")
	if err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if got := agent.Workspace.Files["test_answer.py"]; got != "original" {
		t.Fatalf("deny rule did not protect file; got %q", got)
	}
}

func TestRunTurn_DecisionTimeoutAborts(t *testing.T) {
	pool := newTestPool(t)
	sid := newTestSession(t, pool)
	store := events.NewStore(pool)

	stream := &scriptedStream{
		scripts: [][]NormalizedChunk{
			{
				{Kind: "tool_use_start", ToolUseID: "tu-x", ToolUseName: "FileRead"},
				{Kind: "tool_use_input_delta", ToolUseID: "tu-x", InputJSONDelta: `{"path":"rate_limiter.py"}`},
				{Kind: "tool_use_stop", ToolUseID: "tu-x"},
				{Kind: "message_stop", StopReason: "tool_use"},
			},
		},
	}

	agent := &Agent{
		Stream:    stream,
		Events:    store,
		Waiter:    NewDecisionWaiter(),
		Tools:     NewDefaultRegistry(),
		Workspace: &Workspace{Files: map[string]string{"rate_limiter.py": "x"}},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	res, err := agent.RunTurn(ctx, sid, 1, "read the file")
	if err != nil {
		t.Fatalf("RunTurn: %v (want graceful aborted return, not error)", err)
	}
	if res.Reason != "aborted" {
		t.Fatalf("reason=%q want aborted", res.Reason)
	}
}

func TestRunTurn_MaxIterationsTrips(t *testing.T) {
	pool := newTestPool(t)
	sid := newTestSession(t, pool)
	store := events.NewStore(pool)
	waiter := NewDecisionWaiter()

	// Build a script that keeps proposing FileRead forever; we cap MaxIterations to 3.
	loop := []NormalizedChunk{
		{Kind: "tool_use_start", ToolUseID: "", ToolUseName: "FileRead"},
		{Kind: "tool_use_input_delta", ToolUseID: "", InputJSONDelta: `{"path":"rate_limiter.py"}`},
		{Kind: "tool_use_stop", ToolUseID: ""},
		{Kind: "message_stop", StopReason: "tool_use"},
	}
	makeScript := func(id string) []NormalizedChunk {
		out := make([]NormalizedChunk, len(loop))
		for i := range loop {
			c := loop[i]
			c.ToolUseID = id
			out[i] = c
		}
		return out
	}
	stream := &scriptedStream{scripts: [][]NormalizedChunk{
		makeScript("tu-loop-1"),
		makeScript("tu-loop-2"),
		makeScript("tu-loop-3"),
		makeScript("tu-loop-4"),
	}}

	agent := &Agent{
		Stream:        stream,
		Events:        store,
		Waiter:        waiter,
		Tools:         NewDefaultRegistry(),
		Workspace:     &Workspace{Files: map[string]string{"rate_limiter.py": "x"}},
		MaxIterations: 3,
	}

	// Approve everything so the loop keeps going until MaxIterations trips.
	stopCh := make(chan struct{})
	go func() {
		for {
			select {
			case <-stopCh:
				return
			default:
			}
			for _, id := range []string{"tu-loop-1", "tu-loop-2", "tu-loop-3", "tu-loop-4"} {
				if waiter.Pending(DecisionKey(sid, id)) {
					waiter.Notify(DecisionKey(sid, id), Decision{Kind: "approve"})
				}
			}
			time.Sleep(2 * time.Millisecond)
		}
	}()
	defer close(stopCh)

	res, err := agent.RunTurn(context.Background(), sid, 1, "loop")
	if err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if res.Reason != "max_turns" {
		t.Fatalf("reason=%q want max_turns", res.Reason)
	}
}

func TestRunTurn_NoOpFileEditBypassed(t *testing.T) {
	pool := newTestPool(t)
	sid := newTestSession(t, pool)
	store := events.NewStore(pool)
	waiter := NewDecisionWaiter()

	// Model proposes FileEdit whose content equals the current workspace file.
	// Expectation: handleToolUse short-circuits — no tool_use_proposed event,
	// no waiter registration, tool_result error appended; agent's next turn
	// can see the error and react.
	stream := &scriptedStream{
		scripts: [][]NormalizedChunk{
			{
				{Kind: "tool_use_start", ToolUseID: "tu-noop", ToolUseName: "FileEdit"},
				{Kind: "tool_use_input_delta", ToolUseID: "tu-noop", InputJSONDelta: `{"path":"rate_limiter.py","content":"starter"}`},
				{Kind: "tool_use_stop", ToolUseID: "tu-noop"},
				{Kind: "message_stop", StopReason: "tool_use"},
			},
			{
				{Kind: "text_delta", Text: "Sorry, I'll try a real edit next time."},
				{Kind: "message_stop", StopReason: "end_turn"},
			},
		},
	}

	agent := &Agent{
		Stream:    stream,
		Events:    store,
		Waiter:    waiter,
		Tools:     NewDefaultRegistry(),
		Workspace: &Workspace{Files: map[string]string{"rate_limiter.py": "starter"}},
	}

	// Guard: if the waiter ever sees a Wait for tu-noop, the test fails.
	go func() {
		for i := 0; i < 60; i++ {
			if waiter.Pending(DecisionKey(sid, "tu-noop")) {
				t.Errorf("waiter received tu-noop — short-circuit should have skipped propose+wait")
				waiter.Notify(DecisionKey(sid, "tu-noop"), Decision{Kind: "approve"})
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()

	res, err := agent.RunTurn(context.Background(), sid, 1, "edit rate_limiter")
	if err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if res.Reason != "normal" {
		t.Fatalf("reason=%q want normal", res.Reason)
	}
	if res.Iterations != 2 {
		t.Fatalf("iterations=%d want 2", res.Iterations)
	}
	if got := agent.Workspace.Files["rate_limiter.py"]; got != "starter" {
		t.Fatalf("workspace mutated for no-op edit; got %q", got)
	}

	kinds := collectEvents(t, pool, sid)
	for _, banned := range []string{"tool_use_proposed", "candidate_approved", "candidate_rejected"} {
		if contains(kinds, banned) {
			t.Fatalf("event %q must not fire for no-op edit; got %v", banned, kinds)
		}
	}
	if !contains(kinds, "turn_completed") {
		t.Fatalf("turn_completed missing; got %v", kinds)
	}
}

func TestRunTurn_EmptyFileEditBypassed(t *testing.T) {
	pool := newTestPool(t)
	sid := newTestSession(t, pool)
	store := events.NewStore(pool)
	waiter := NewDecisionWaiter()

	stream := &scriptedStream{
		scripts: [][]NormalizedChunk{
			{
				{Kind: "tool_use_start", ToolUseID: "tu-empty", ToolUseName: "FileEdit"},
				{
					Kind:           "tool_use_input_delta",
					ToolUseID:      "tu-empty",
					InputJSONDelta: `{"path":"rate_limiter.py","content":""}`,
				},
				{Kind: "tool_use_stop", ToolUseID: "tu-empty"},
				{Kind: "message_stop", StopReason: "tool_use"},
			},
			{
				{Kind: "text_delta", Text: "I will retry with non-empty content."},
				{Kind: "message_stop", StopReason: "end_turn"},
			},
		},
	}

	agent := &Agent{
		Stream: stream,
		Events: store,
		Waiter: waiter,
		Tools:  NewDefaultRegistry(),
		Workspace: &Workspace{Files: map[string]string{
			"rate_limiter.py": "starter",
		}},
	}

	go func() {
		for i := 0; i < 60; i++ {
			if waiter.Pending("tu-empty") {
				t.Errorf("empty FileEdit entered the approval flow")
				waiter.Notify("tu-empty", Decision{Kind: "reject"})
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()

	res, err := agent.RunTurn(
		context.Background(),
		sid,
		1,
		"edit rate_limiter.py",
	)
	if err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if res.Reason != "normal" {
		t.Fatalf("reason=%q want normal", res.Reason)
	}
	if res.Iterations != 2 {
		t.Fatalf("iterations=%d want 2", res.Iterations)
	}
	if got := agent.Workspace.Files["rate_limiter.py"]; got != "starter" {
		t.Fatalf("workspace changed after empty edit; got %q", got)
	}

	kinds := collectEvents(t, pool, sid)
	for _, banned := range []string{
		"tool_use_proposed",
		"candidate_approved",
		"candidate_rejected",
	} {
		if contains(kinds, banned) {
			t.Fatalf(
				"event %q must not fire for empty FileEdit; got %v",
				banned,
				kinds,
			)
		}
	}
}

func TestRunTurn_FileReadAutoApproved(t *testing.T) {
	pool := newTestPool(t)
	sid := newTestSession(t, pool)
	store := events.NewStore(pool)
	waiter := NewDecisionWaiter()

	// Model proposes FileRead — read-class tool. Expectation: the runtime
	// auto-approves (no waiter Wait call), emits tool_use_proposed +
	// candidate_approved both flagged auto, executes the read, and
	// continues to the next turn where the model wraps up.
	stream := &scriptedStream{
		scripts: [][]NormalizedChunk{
			{
				{Kind: "tool_use_start", ToolUseID: "tu-read", ToolUseName: "FileRead"},
				{Kind: "tool_use_input_delta", ToolUseID: "tu-read", InputJSONDelta: `{"path":"rate_limiter.py"}`},
				{Kind: "tool_use_stop", ToolUseID: "tu-read"},
				{Kind: "message_stop", StopReason: "tool_use"},
			},
			{
				{Kind: "text_delta", Text: "I read the file."},
				{Kind: "message_stop", StopReason: "end_turn"},
			},
		},
	}

	agent := &Agent{
		Stream:    stream,
		Events:    store,
		Waiter:    waiter,
		Tools:     NewDefaultRegistry(),
		Workspace: &Workspace{Files: map[string]string{"rate_limiter.py": "print('hello')"}},
	}

	// Guard: waiter must never see tu-read because FileRead is auto-approved.
	go func() {
		for i := 0; i < 60; i++ {
			if waiter.Pending(DecisionKey(sid, "tu-read")) {
				t.Errorf("waiter received tu-read — FileRead should have been auto-approved")
				waiter.Notify(DecisionKey(sid, "tu-read"), Decision{Kind: "approve"})
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()

	res, err := agent.RunTurn(context.Background(), sid, 1, "read the file")
	if err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if res.Reason != "normal" {
		t.Fatalf("reason=%q want normal", res.Reason)
	}
	if res.Iterations != 2 {
		t.Fatalf("iterations=%d want 2", res.Iterations)
	}

	kinds := collectEvents(t, pool, sid)
	for _, want := range []string{"tool_use_proposed", "candidate_approved", "tool_result", "turn_completed"} {
		if !contains(kinds, want) {
			t.Fatalf("missing event %q in %v", want, kinds)
		}
	}

	// Verify Auto=true on the persisted candidate_approved row.
	var approvedAuto bool
	err = pool.QueryRow(context.Background(),
		`SELECT (payload->>'auto')::bool FROM session_events
		 WHERE session_id=$1 AND kind='candidate_approved' ORDER BY seq DESC LIMIT 1`,
		sid).Scan(&approvedAuto)
	if err != nil {
		t.Fatalf("query approved row: %v", err)
	}
	if !approvedAuto {
		t.Fatalf("candidate_approved.auto = false; expected true for FileRead")
	}
}

func contains(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}

func TestRunTurn_StopsRepeatedIdenticalRead(t *testing.T) {
	pool := newTestPool(t)
	sid := newTestSession(t, pool)
	store := events.NewStore(pool)

	repeatedRead := func(id string) []NormalizedChunk {
		return []NormalizedChunk{
			{
				Kind:        "tool_use_start",
				ToolUseID:   id,
				ToolUseName: "FileRead",
			},
			{
				Kind:           "tool_use_input_delta",
				ToolUseID:      id,
				InputJSONDelta: `{"path":"answer.py"}`,
			},
			{
				Kind:      "tool_use_stop",
				ToolUseID: id,
			},
			{
				Kind:       "message_stop",
				StopReason: "tool_use",
			},
		}
	}

	stream := &scriptedStream{
		scripts: [][]NormalizedChunk{
			repeatedRead("tu-read-1"),
			repeatedRead("tu-read-2"),
			repeatedRead("tu-read-3"),
			repeatedRead("tu-read-4"),
		},
	}

	agent := &Agent{
		Stream: stream,
		Events: store,
		Waiter: NewDecisionWaiter(),
		Tools:  NewDefaultRegistry(),
		Workspace: &Workspace{
			Files: map[string]string{
				"answer.py": "print('hello')",
			},
		},
	}

	result, err := agent.RunTurn(
		context.Background(),
		sid,
		1,
		"inspect the file",
	)
	if err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if result.Reason != "max_turns" {
		t.Fatalf("reason=%q want max_turns", result.Reason)
	}
	if result.Iterations != 4 {
		t.Fatalf("iterations=%d want 4", result.Iterations)
	}

	var proposals int
	err = pool.QueryRow(
		context.Background(),
		`SELECT count(*) FROM session_events
		 WHERE session_id=$1 AND kind='tool_use_proposed'`,
		sid,
	).Scan(&proposals)
	if err != nil {
		t.Fatalf("count proposals: %v", err)
	}
	if proposals != maxRepeatedAutoToolCalls {
		t.Fatalf(
			"proposals=%d want %d",
			proposals,
			maxRepeatedAutoToolCalls,
		)
	}
}

// silence unused-import warnings on errors package without separate file
var _ = errors.New

func TestRunTurn_PersistsTranscript(t *testing.T) {
	pool := newTestPool(t)
	sid := newTestSession(t, pool)
	store := events.NewStore(pool)

	// user → assistant(tool_use FileRead, auto-approved) → tool_result →
	// assistant text. Every history message must land in session_messages.
	stream := &scriptedStream{
		scripts: [][]NormalizedChunk{
			{
				{Kind: "tool_use_start", ToolUseID: "tu-read", ToolUseName: "FileRead"},
				{Kind: "tool_use_input_delta", ToolUseID: "tu-read", InputJSONDelta: `{"path":"rate_limiter.py"}`},
				{Kind: "tool_use_stop", ToolUseID: "tu-read"},
				{Kind: "message_stop", StopReason: "tool_use"},
			},
			{
				{Kind: "text_delta", Text: "I read the file."},
				{Kind: "message_stop", StopReason: "end_turn"},
			},
		},
	}

	agent := &Agent{
		Stream:    stream,
		Events:    store,
		Waiter:    NewDecisionWaiter(),
		Tools:     NewDefaultRegistry(),
		Workspace: &Workspace{Files: map[string]string{"rate_limiter.py": "print('hello')"}},
	}

	if _, err := agent.RunTurn(context.Background(), sid, 1, "read the file"); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}

	rows, err := pool.Query(context.Background(),
		`SELECT role, content::text FROM session_messages WHERE session_id=$1 ORDER BY seq`, sid)
	if err != nil {
		t.Fatalf("query messages: %v", err)
	}
	defer rows.Close()
	var roles, contents []string
	for rows.Next() {
		var role, content string
		if err := rows.Scan(&role, &content); err != nil {
			t.Fatalf("scan: %v", err)
		}
		roles = append(roles, role)
		contents = append(contents, content)
	}

	wantRoles := []string{"user", "assistant", "tool", "assistant"}
	if fmt.Sprint(roles) != fmt.Sprint(wantRoles) {
		t.Fatalf("roles=%v want %v", roles, wantRoles)
	}
	if !strings.Contains(contents[0], "read the file") {
		t.Fatalf("user message not persisted: %s", contents[0])
	}
	if !strings.Contains(contents[1], `"tool_use"`) || !strings.Contains(contents[1], "FileRead") {
		t.Fatalf("assistant tool_use not persisted: %s", contents[1])
	}
	if !strings.Contains(contents[3], "I read the file.") {
		t.Fatalf("final assistant text not persisted: %s", contents[3])
	}

	// Transcript and event seqs come from one counter, so they never collide.
	var dup int
	if err := pool.QueryRow(context.Background(), `
		SELECT count(*) FROM session_messages m
		JOIN session_events e ON e.session_id = m.session_id AND e.seq = m.seq
		WHERE m.session_id = $1`, sid).Scan(&dup); err != nil {
		t.Fatalf("seq overlap query: %v", err)
	}
	if dup != 0 {
		t.Fatalf("%d transcript rows share a seq with an event", dup)
	}
}

func TestAnthropicBlocks(t *testing.T) {
	got := anthropicBlocks([]ContentBlock{
		{Kind: "text", Text: "hi"},
		{Kind: "tool_use", ToolUse: &ContentToolUse{ID: "tu-1", Name: "FileRead", Input: map[string]any{"path": "a.py"}}},
		{Kind: "tool_result", ToolResult: &ContentToolResult{ToolUseID: "tu-1", Output: "ok", IsError: true}},
		{Kind: "tool_use"}, // missing payload: skipped
	})
	if len(got) != 3 {
		t.Fatalf("len=%d want 3: %v", len(got), got)
	}
	if got[0]["type"] != "text" || got[0]["text"] != "hi" {
		t.Fatalf("text block = %v", got[0])
	}
	if got[1]["type"] != "tool_use" || got[1]["name"] != "FileRead" || got[1]["id"] != "tu-1" {
		t.Fatalf("tool_use block = %v", got[1])
	}
	if got[2]["type"] != "tool_result" || got[2]["tool_use_id"] != "tu-1" || got[2]["content"] != "ok" || got[2]["is_error"] != true {
		t.Fatalf("tool_result block = %v", got[2])
	}
}

func TestSummarizeInput_FileEditUsesNewText(t *testing.T) {
	got := summarizeInput("FileEdit", map[string]any{
		"path":     "answer.py",
		"old_text": "before",
		"new_text": "after",
	})

	if got != "Edit answer.py (5 bytes)" {
		t.Fatalf("summary=%q", got)
	}
}
