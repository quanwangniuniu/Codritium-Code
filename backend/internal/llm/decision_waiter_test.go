package llm

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestDecisionWaiter_WaitThenNotify(t *testing.T) {
	w := NewDecisionWaiter()
	got := make(chan Decision, 1)

	go func() {
		d, err := w.Wait(context.Background(), "tu-1")
		if err != nil {
			t.Errorf("wait: %v", err)
			return
		}
		got <- d
	}()

	// Give Wait a chance to register
	deadline := time.Now().Add(500 * time.Millisecond)
	for !w.Pending("tu-1") {
		if time.Now().After(deadline) {
			t.Fatal("Wait did not register pending entry")
		}
		time.Sleep(2 * time.Millisecond)
	}

	if !w.Notify("tu-1", Decision{Kind: "approve"}) {
		t.Fatal("Notify returned false for a pending id")
	}

	select {
	case d := <-got:
		if d.Kind != "approve" {
			t.Fatalf("kind=%q want approve", d.Kind)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("did not receive decision")
	}

	if w.Pending("tu-1") {
		t.Fatal("entry not cleaned up after Notify")
	}
}

func TestDecisionWaiter_NotifyWithoutWaitReturnsFalse(t *testing.T) {
	w := NewDecisionWaiter()
	if w.Notify("nobody-waiting", Decision{Kind: "approve"}) {
		t.Fatal("Notify returned true for an id with no pending Wait")
	}
}

func TestDecisionWaiter_ContextCancel(t *testing.T) {
	w := NewDecisionWaiter()
	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		_, err := w.Wait(ctx, "tu-2")
		errCh <- err
	}()

	deadline := time.Now().Add(500 * time.Millisecond)
	for !w.Pending("tu-2") {
		if time.Now().After(deadline) {
			t.Fatal("Wait did not register")
		}
		time.Sleep(2 * time.Millisecond)
	}

	cancel()

	select {
	case err := <-errCh:
		if err != context.Canceled {
			t.Fatalf("err=%v want Canceled", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Wait did not return on cancel")
	}

	// A late Notify after cancel must not panic and must report false.
	if w.Notify("tu-2", Decision{Kind: "approve"}) {
		t.Fatal("Notify returned true after Wait was cancelled")
	}
}

func TestDecisionWaiter_DuplicateWaitErrors(t *testing.T) {
	w := NewDecisionWaiter()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_, _ = w.Wait(ctx, "tu-dup")
	}()
	// Spin until first Wait is registered
	for !w.Pending("tu-dup") {
		time.Sleep(time.Millisecond)
	}

	_, err := w.Wait(ctx, "tu-dup")
	if err != ErrDuplicate {
		t.Fatalf("err=%v want ErrDuplicate", err)
	}
}

func TestDecisionWaiter_ConcurrentDistinctIDs(t *testing.T) {
	w := NewDecisionWaiter()
	const n = 30

	var wg sync.WaitGroup
	var got atomic.Int32

	for i := 0; i < n; i++ {
		id := "tu-" + idForTest(i)
		wg.Add(1)
		go func() {
			defer wg.Done()
			d, err := w.Wait(context.Background(), id)
			if err == nil && d.Kind == "approve" {
				got.Add(1)
			}
		}()
	}

	// Wait for all to register, then notify each
	deadline := time.Now().Add(2 * time.Second)
	for {
		count := 0
		for i := 0; i < n; i++ {
			if w.Pending("tu-" + idForTest(i)) {
				count++
			}
		}
		if count == n {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("only %d/%d waits registered", count, n)
		}
		time.Sleep(time.Millisecond)
	}

	for i := 0; i < n; i++ {
		ok := w.Notify("tu-"+idForTest(i), Decision{Kind: "approve"})
		if !ok {
			t.Fatalf("notify %d returned false", i)
		}
	}

	wg.Wait()
	if got.Load() != int32(n) {
		t.Fatalf("got %d/%d approvals", got.Load(), n)
	}
}

func idForTest(i int) string {
	return string(rune('a'+(i%26))) + string(rune('a'+((i/26)%26)))
}

func TestDecision_IsValid(t *testing.T) {
	cases := []struct {
		kind string
		ok   bool
	}{
		{"approve", true},
		{"reject", true},
		{"modify", true},
		{"", false},
		{"yes", false},
		{"APPROVE", false},
	}
	for _, c := range cases {
		if (Decision{Kind: c.kind}).IsValid() != c.ok {
			t.Errorf("IsValid(%q)=%v want %v", c.kind, !c.ok, c.ok)
		}
	}
}
