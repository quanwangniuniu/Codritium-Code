package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"codritium/backend/internal/auth"
	"codritium/backend/internal/events"
	"codritium/backend/internal/platform/testutil"
)

func TestGetMyReplayCombinesMessagesAndEventsInSequence(t *testing.T) {
	pool := testutil.Pool(t)
	handle := "replay-" + uuid.NewString()
	sessionID := createSessionFor(t, pool, handle)
	store := events.NewStore(pool)
	ctx := context.Background()

	if err := store.AppendMessage(
		ctx,
		sessionID,
		"user",
		[]byte(`[{"type":"text","text":"Read main.py and fix the bug."}]`),
	); err != nil {
		t.Fatalf("append user message: %v", err)
	}

	if err := store.AppendMessage(
		ctx,
		sessionID,
		"assistant",
		[]byte(`[{"type":"tool_use","id":"tool-1","name":"FileEdit","input":{"path":"main.py","content":"print(\"fixed\")"}}]`),
	); err != nil {
		t.Fatalf("append assistant tool call: %v", err)
	}

	if _, err := store.Append(ctx, sessionID, events.ToolUseProposed{
		ToolUseID:    "tool-1",
		Tool:         "FileEdit",
		InputSummary: "Edit main.py",
		TurnIndex:    1,
	}); err != nil {
		t.Fatalf("append tool proposal: %v", err)
	}

	if _, err := store.Append(ctx, sessionID, events.CandidateApproved{
		ToolUseID: "tool-1",
		Modified:  false,
	}); err != nil {
		t.Fatalf("append candidate approval: %v", err)
	}

	if err := store.AppendMessage(
		ctx,
		sessionID,
		"tool",
		[]byte(`[{"type":"tool_result","tool_use_id":"tool-1","content":"Updated main.py","is_error":false}]`),
	); err != nil {
		t.Fatalf("append tool result: %v", err)
	}

	if err := store.AppendMessage(
		ctx,
		sessionID,
		"assistant",
		[]byte(`[{"type":"text","text":"The file has been updated."}]`),
	); err != nil {
		t.Fatalf("append assistant reply: %v", err)
	}

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/me/replays/22-build-rate-limiter-middleware",
		nil,
	)
	req.SetPathValue("slug", "22-build-rate-limiter-middleware")
	req = req.WithContext(auth.WithUser(req.Context(), &auth.User{
		ID:          uuid.New(),
		Handle:      handle,
		DisplayName: handle,
	}))

	recorder := httptest.NewRecorder()
	GetMyReplay(ReplyDeps{Pool: pool}).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf(
			"status = %d, want 200; body = %s",
			recorder.Code,
			recorder.Body.String(),
		)
	}

	var response replyResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.SessionID != sessionID.String() {
		t.Fatalf(
			"session_id = %q, want %q",
			response.SessionID,
			sessionID,
		)
	}

	wantKinds := []string{
		"chat_message",
		"chat_message",
		"tool_use_proposed",
		"candidate_approved",
		"chat_message",
		"chat_message",
	}

	if len(response.Envelopes) != len(wantKinds) {
		t.Fatalf(
			"envelope count = %d, want %d; body = %s",
			len(response.Envelopes),
			len(wantKinds),
			recorder.Body.String(),
		)
	}

	var previousSeq int64
	for index, raw := range response.Envelopes {
		var envelope userEnvelope
		if err := json.Unmarshal(raw, &envelope); err != nil {
			t.Fatalf("decode envelope %d: %v", index, err)
		}

		if envelope.Kind != wantKinds[index] {
			t.Errorf(
				"envelope %d kind = %q, want %q",
				index,
				envelope.Kind,
				wantKinds[index],
			)
		}

		if envelope.Seq <= previousSeq {
			t.Errorf(
				"envelope %d seq = %d, previous seq = %d",
				index,
				envelope.Seq,
				previousSeq,
			)
		}
		previousSeq = envelope.Seq
	}

	var firstMessage struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	}
	var firstEnvelope userEnvelope
	if err := json.Unmarshal(response.Envelopes[0], &firstEnvelope); err != nil {
		t.Fatalf("decode first envelope: %v", err)
	}
	if err := json.Unmarshal(firstEnvelope.Payload, &firstMessage); err != nil {
		t.Fatalf("decode first message: %v", err)
	}

	if firstMessage.Role != "user" {
		t.Fatalf("first message role = %q, want user", firstMessage.Role)
	}

	if string(firstMessage.Content) !=
		`[{"text":"Read main.py and fix the bug.","type":"text"}]` {
		t.Fatalf(
			"first message content = %s",
			firstMessage.Content,
		)
	}
}
