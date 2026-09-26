package handlers

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"codritium/backend/internal/auth"
	"codritium/backend/internal/llm"
)

func authedRequest(t *testing.T, method, path, body string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	u := &auth.User{ID: uuid.New(), Handle: "alice", DisplayName: "Alice Wang"}
	return req.WithContext(auth.WithUser(req.Context(), u))
}

func TestPostDecision_Unauthorized(t *testing.T) {
	waiter := llm.NewDecisionWaiter()
	h := PostDecision(DecisionDeps{Waiter: waiter})

	req := httptest.NewRequest(http.MethodPost, "/api/decision", strings.NewReader(`{}`))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d want 401", rr.Code)
	}
}

func TestPostDecision_BadJSON(t *testing.T) {
	h := PostDecision(DecisionDeps{Waiter: llm.NewDecisionWaiter()})
	req := authedRequest(t, http.MethodPost, "/api/decision", `{not json`)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want 400", rr.Code)
	}
}

func TestPostDecision_MissingToolUseID(t *testing.T) {
	h := PostDecision(DecisionDeps{Waiter: llm.NewDecisionWaiter()})
	req := authedRequest(t, http.MethodPost, "/api/decision", `{"decision":"approve"}`)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want 400", rr.Code)
	}
}

func TestPostDecision_BadDecisionKind(t *testing.T) {
	h := PostDecision(DecisionDeps{Waiter: llm.NewDecisionWaiter()})
	req := authedRequest(t, http.MethodPost, "/api/decision",
		`{"tool_use_id":"tu-1","decision":"yes"}`)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want 400", rr.Code)
	}
}

func TestPostDecision_NoPendingWaitReturns404(t *testing.T) {
	h := PostDecision(DecisionDeps{Waiter: llm.NewDecisionWaiter()})
	req := authedRequest(t, http.MethodPost, "/api/decision",
		`{"tool_use_id":"tu-not-waiting","decision":"approve"}`)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status=%d want 404", rr.Code)
	}
}

func TestPostDecision_RoundTrip(t *testing.T) {
	waiter := llm.NewDecisionWaiter()
	h := PostDecision(DecisionDeps{Waiter: waiter})

	// Start a Wait in a goroutine; capture the decision it receives.
	got := make(chan llm.Decision, 1)
	go func() {
		d, err := waiter.Wait(context.Background(), "tu-real")
		if err != nil {
			t.Errorf("wait: %v", err)
			return
		}
		got <- d
	}()

	// Wait for the entry to register.
	deadline := time.Now().Add(500 * time.Millisecond)
	for !waiter.Pending("tu-real") {
		if time.Now().After(deadline) {
			t.Fatal("Wait did not register")
		}
		time.Sleep(2 * time.Millisecond)
	}

	body := bytes.NewBufferString(`{
		"tool_use_id":"tu-real",
		"decision":"modify",
		"modified_input":"{\"path\":\"x.py\"}",
		"comment":"smaller patch please"
	}`)
	req := httptest.NewRequest(http.MethodPost, "/api/decision", body)
	u := &auth.User{ID: uuid.New(), Handle: "alice", DisplayName: "Alice Wang"}
	req = req.WithContext(auth.WithUser(req.Context(), u))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status=%d want 204; body=%s", rr.Code, rr.Body.String())
	}

	select {
	case d := <-got:
		if d.Kind != "modify" {
			t.Fatalf("kind=%q", d.Kind)
		}
		if d.ModifiedInput != `{"path":"x.py"}` {
			t.Fatalf("modified_input=%q", d.ModifiedInput)
		}
		if d.Comment != "smaller patch please" {
			t.Fatalf("comment=%q", d.Comment)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("waiter did not receive decision")
	}
}
