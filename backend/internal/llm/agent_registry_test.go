package llm

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
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
