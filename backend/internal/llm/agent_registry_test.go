package llm

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"codritium/backend/internal/events"
)

func TestAgentRegistry_DropRemovesAgentAndTmpFS(t *testing.T) {
	sid := uuid.New()
	var builds int
	reg := NewAgentRegistry(func(ctx context.Context, sessionID uuid.UUID, slug string) (*Agent, error) {
		builds++
		tmp, err := NewSessionTmpFS(sessionID, nil)
		if err != nil {
			return nil, err
		}
		return &Agent{TmpFS: tmp}, nil
	})

	a, err := reg.GetOrCreate(context.Background(), sid, "slug")
	if err != nil {
		t.Fatalf("GetOrCreate: %v", err)
	}
	dir := a.TmpFS.Dir()
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("tmpfs dir missing before Drop: %v", err)
	}

	reg.Drop(sid)

	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("tmpfs dir still present after Drop (stat err=%v)", err)
	}
	if _, err := reg.GetOrCreate(context.Background(), sid, "slug"); err != nil {
		t.Fatalf("GetOrCreate after Drop: %v", err)
	}
	if builds != 2 {
		t.Fatalf("factory builds=%d want 2 (Drop should evict the cached agent)", builds)
	}

	reg.Drop(uuid.New()) // unknown session: no-op
}

func TestAgentRegistry_DropCancelsBlockedTurn(t *testing.T) {
	pool := newTestPool(t)
	sid := newTestSession(t, pool)
	waiter := NewDecisionWaiter()

	// The model proposes a FileEdit, so RunTurn blocks waiting for the
	// candidate's decision — which never comes. Drop must unblock it.
	agent := &Agent{
		Stream: &scriptedStream{scripts: [][]NormalizedChunk{{
			{Kind: "tool_use_start", ToolUseID: "tu-block", ToolUseName: "FileEdit"},
			{Kind: "tool_use_input_delta", ToolUseID: "tu-block", InputJSONDelta: `{"path":"a.py","content":"x"}`},
			{Kind: "tool_use_stop", ToolUseID: "tu-block"},
			{Kind: "message_stop", StopReason: "tool_use"},
		}}},
		Events:    events.NewStore(pool),
		Waiter:    waiter,
		Tools:     NewDefaultRegistry(),
		Workspace: &Workspace{Files: map[string]string{"a.py": "starter"}},
	}
	reg := NewAgentRegistry(func(context.Context, uuid.UUID, string) (*Agent, error) { return agent, nil })
	if _, err := reg.GetOrCreate(context.Background(), sid, "slug"); err != nil {
		t.Fatalf("GetOrCreate: %v", err)
	}

	done := make(chan RunResult, 1)
	go func() {
		res, _ := agent.RunTurn(context.Background(), sid, 1, "edit it")
		done <- res
	}()
	deadline := time.Now().Add(2 * time.Second)
	for !waiter.Pending(DecisionKey(sid, "tu-block")) {
		if time.Now().After(deadline) {
			t.Fatal("turn never blocked on the decision")
		}
		time.Sleep(5 * time.Millisecond)
	}

	reg.Drop(sid)

	select {
	case res := <-done:
		if res.Reason != "aborted" {
			t.Fatalf("reason=%q want aborted", res.Reason)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("RunTurn still blocked after Drop")
	}
}

func TestSessionTmpFS_NoSyncAfterCleanup(t *testing.T) {
	tmp, err := NewSessionTmpFS(uuid.New(), nil)
	if err != nil {
		t.Fatalf("NewSessionTmpFS: %v", err)
	}
	dir := tmp.Dir()
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	if err := tmp.Cleanup(); err != nil {
		t.Fatalf("Cleanup: %v", err)
	}
	if err := tmp.Sync(map[string]string{"a.py": "x"}); err == nil {
		t.Fatal("Sync after Cleanup succeeded; want error")
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("Sync recreated the cleaned-up dir (stat err=%v)", err)
	}
}
