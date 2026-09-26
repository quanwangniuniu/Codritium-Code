package tips

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestAgent_RejectsNonPracticeMode(t *testing.T) {
	a := &Agent{}
	for _, mode := range []Mode{ModeSimulator, ModeReply, Mode("")} {
		_, err := a.Ask(context.Background(), Request{
			Mode:         mode,
			Conversation: []Turn{{Role: "user", Text: "hi"}},
		})
		if err == nil {
			t.Fatalf("expected mode %q to be rejected", mode)
		}
		if !strings.Contains(err.Error(), "mode") {
			t.Fatalf("expected mode error for %q, got %v", mode, err)
		}
	}
}

func TestAgent_RejectsEmptyConversation(t *testing.T) {
	a := &Agent{}
	_, err := a.Ask(context.Background(), Request{
		Mode: ModePractice,
	})
	if err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("expected empty conversation error, got %v", err)
	}
}

func TestAgent_RejectsNilClient(t *testing.T) {
	var a *Agent
	_, err := a.Ask(context.Background(), Request{Mode: ModePractice, Conversation: []Turn{{Role: "user", Text: "hi"}}})
	if !errors.Is(err, err) || err == nil {
		t.Fatal("expected error from nil agent")
	}
}
