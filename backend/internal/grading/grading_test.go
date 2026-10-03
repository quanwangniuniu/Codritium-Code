package grading

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"codritium/backend/internal/grader"
	"codritium/backend/internal/platform/testutil"
)

func TestFlattenBlocks(t *testing.T) {
	raw := []byte(`[{"type":"text","text":"why twice?"},{"type":"tool_use","name":"FileRead","input":{}},{"type":"tool_result","content":"huge"}]`)
	if got := flattenBlocks(raw); got != "why twice?\n<tool:FileRead>" {
		t.Fatalf("got %q", got)
	}
	if got := flattenBlocks([]byte(`"plain"`)); got != "plain" {
		t.Fatalf("string: %q", got)
	}
	if got := flattenBlocks([]byte(`{bad`)); got != "" {
		t.Fatalf("bad: %q", got)
	}
}

func TestRetry(t *testing.T) {
	calls := 0
	err := Retry(context.Background(), 3, time.Millisecond, func() error {
		calls++
		if calls < 3 {
			return errors.New("flaky")
		}
		return nil
	})
	if err != nil || calls != 3 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	calls = 0
	if err := Retry(ctx, 5, time.Hour, func() error { calls++; return errors.New("x") }); err == nil || calls != 1 {
		t.Fatalf("cancelled: err=%v calls=%d", err, calls)
	}
}

type blockingSandbox struct{ started chan struct{} }

func (b blockingSandbox) RunPytest(ctx context.Context, _ grader.SandboxInput) (*grader.SandboxOutput, error) {
	close(b.started)
	<-ctx.Done()
	return nil, ctx.Err()
}

type okSandbox struct{}

func (okSandbox) RunPytest(context.Context, grader.SandboxInput) (*grader.SandboxOutput, error) {
	return &grader.SandboxOutput{Status: "ok", PassCount: 2, Total: 2}, nil
}

type okEngine struct{}

func (okEngine) Name() string { return "fake" }
func (okEngine) Grade(context.Context, grader.GraderInput) (*grader.GraderResult, error) {
	return &grader.GraderResult{FinalScore: 80}, nil
}

func newSubmission(t *testing.T) uuid.UUID {
	t.Helper()
	pool := testutil.Pool(t)
	u := testutil.NewUser(t, pool, "user")
	var id uuid.UUID
	if err := pool.QueryRow(context.Background(), `
		INSERT INTO submissions (user_id, problem_id, status)
		SELECT $1, id, 'grading' FROM problems ORDER BY slug LIMIT 1
		RETURNING id`, u.ID).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func status(t *testing.T, id uuid.UUID) (string, *float64) {
	t.Helper()
	var st string
	var score *float64
	if err := testutil.Pool(t).QueryRow(context.Background(),
		`SELECT status, final_score::float8 FROM submissions WHERE id = $1`, id).Scan(&st, &score); err != nil {
		t.Fatal(err)
	}
	return st, score
}

func TestService_GradesAndPersists(t *testing.T) {
	id := newSubmission(t)
	s := &Service{Pool: testutil.Pool(t), Sandbox: okSandbox{}, Engine: okEngine{}}
	if err := s.Enqueue(Job{SubmissionID: id}); err != nil {
		t.Fatal(err)
	}
	if err := s.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	if st, score := status(t, id); st != "graded" || score == nil || *score != 80 {
		t.Fatalf("status=%s score=%v", st, score)
	}
}

// A job still running when the shutdown deadline passes is cancelled and
// marked failed, not left in "grading" forever.
func TestService_ShutdownCancelsAndMarksFailed(t *testing.T) {
	id := newSubmission(t)
	sb := blockingSandbox{started: make(chan struct{})}
	s := &Service{Pool: testutil.Pool(t), Sandbox: sb, Engine: okEngine{}}
	if err := s.Enqueue(Job{SubmissionID: id}); err != nil {
		t.Fatal(err)
	}
	<-sb.started
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := s.Shutdown(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("shutdown err=%v", err)
	}
	if st, _ := status(t, id); st != "failed" {
		t.Fatalf("status=%s want failed", st)
	}
	if err := s.Enqueue(Job{SubmissionID: id}); !errors.Is(err, ErrShuttingDown) {
		t.Fatalf("enqueue after shutdown: %v", err)
	}
}
