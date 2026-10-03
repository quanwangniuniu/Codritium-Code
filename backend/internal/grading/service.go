// Package grading runs submissions through the sandbox and the rubric
// grader in tracked background jobs.
package grading

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"codritium/backend/internal/grader"
)

// JobTimeout bounds one grading run end to end.
const JobTimeout = 5 * time.Minute

// Sandbox runs the hidden tests.
type Sandbox interface {
	RunPytest(ctx context.Context, input grader.SandboxInput) (*grader.SandboxOutput, error)
}

// Job is everything needed to grade one submission.
type Job struct {
	SubmissionID      uuid.UUID
	ProblemTitle      string
	Difficulty        string
	ReadmeMD          string
	Starter           map[string]string
	Candidate         map[string]string
	HiddenTestFile    string
	HiddenTestContent string
}

// Service grades submissions in the background. Unlike the old detached
// goroutine, its jobs are tracked: Shutdown waits for them and, if the
// deadline passes, cancels them and marks the submissions failed instead
// of leaving them stuck in "grading".
type Service struct {
	Pool    *pgxpool.Pool
	Sandbox Sandbox
	Engine  grader.Engine

	once   sync.Once
	base   context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	mu     sync.Mutex
	closed bool
}

func (s *Service) init() {
	s.once.Do(func() { s.base, s.cancel = context.WithCancel(context.Background()) })
}

// ErrShuttingDown is returned by Enqueue after Shutdown started.
var ErrShuttingDown = errors.New("grading: shutting down")

// Enqueue starts grading job in the background.
func (s *Service) Enqueue(job Job) error {
	s.init()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrShuttingDown
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.run(job)
	}()
	return nil
}

// Shutdown stops accepting jobs and waits for running ones until ctx is
// done, then cancels whatever is left.
func (s *Service) Shutdown(ctx context.Context) error {
	s.init()
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()
	done := make(chan struct{})
	go func() { s.wg.Wait(); close(done) }()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		s.cancel()
		<-done
		return ctx.Err()
	}
}

func (s *Service) run(job Job) {
	ctx, cancel := context.WithTimeout(s.base, JobTimeout)
	defer cancel()
	id := job.SubmissionID

	var sb *grader.SandboxOutput
	err := Retry(ctx, 3, time.Second, func() error {
		var e error
		sb, e = s.Sandbox.RunPytest(ctx, grader.SandboxInput{
			StarterFiles:       job.Starter,
			CandidateFiles:     job.Candidate,
			HiddenTestFilename: job.HiddenTestFile,
			HiddenTestContent:  job.HiddenTestContent,
			TimeoutSec:         90,
		})
		return e
	})
	if err != nil {
		s.fail(id, "sandbox", err)
		return
	}

	history, err := loadPromptHistory(ctx, s.Pool, id)
	if err != nil {
		// Grade without the transcript rather than failing the submission.
		log.Printf("[grade %s] load history: %v", id, err)
	}
	res, err := s.Engine.Grade(ctx, grader.GraderInput{
		ProblemTitle:      job.ProblemTitle,
		ProblemDifficulty: job.Difficulty,
		ProblemReadme:     job.ReadmeMD,
		StarterFiles:      job.Starter,
		CandidateFiles:    job.Candidate,
		PromptHistory:     history,
		TestResults:       sb,
	})
	if err != nil {
		s.fail(id, "grader", err)
		return
	}

	testJSON, _ := json.Marshal(sb)
	scoresJSON, _ := json.Marshal(res)
	if _, err := s.Pool.Exec(ctx, `
		UPDATE submissions SET test_results = $2::jsonb, scores = $3::jsonb,
		       final_score = $4, status = 'graded', graded_at = now()
		WHERE id = $1`, id, string(testJSON), string(scoresJSON), res.FinalScore,
	); err != nil {
		s.fail(id, "persist", err)
		return
	}
	log.Printf("[grade %s] done engine=%s final=%.1f tests=%d/%d dims=%v",
		id, s.Engine.Name(), res.FinalScore, sb.PassCount, sb.Total, res.EvaluatedDims)
}

// fail marks the submission failed. It uses its own context: the job's may
// already be cancelled (timeout or shutdown), and the row must not stay
// in "grading".
func (s *Service) fail(id uuid.UUID, stage string, err error) {
	log.Printf("[grade %s] %s error: %v", id, stage, err)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	msg, _ := json.Marshal(map[string]string{"error": fmt.Sprintf("%s: %v", stage, err)})
	if _, dbErr := s.Pool.Exec(ctx, `
		UPDATE submissions SET status = 'failed', scores = $2::jsonb, graded_at = now()
		WHERE id = $1`, id, string(msg)); dbErr != nil {
		log.Printf("[grade %s] mark failed: %v", id, dbErr)
	}
}

// Retry runs fn up to attempts times with exponential backoff starting at
// delay, stopping early when ctx is done.
func Retry(ctx context.Context, attempts int, delay time.Duration, fn func() error) error {
	attempts = max(attempts, 1)
	var err error
	for i := 0; i < attempts; i++ {
		if err = fn(); err == nil {
			return nil
		}
		if i == attempts-1 {
			break
		}
		select {
		case <-time.After(delay):
			delay *= 2
		case <-ctx.Done():
			return errors.Join(err, ctx.Err())
		}
	}
	return err
}
