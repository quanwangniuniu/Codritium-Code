package submissions

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"

	"codritium/backend/internal/auth"
	"codritium/backend/internal/grader"
	"codritium/backend/internal/grading"
	"codritium/backend/internal/platform/httpx"
	"codritium/backend/internal/problems"
	"codritium/backend/internal/sessions"
)

// RateLimitPerHour caps new submissions per user per rolling hour. Each
// one burns sandbox time and a five-dimension grader run.
const RateLimitPerHour = 5

// ListLimit caps GET /api/me/submissions.
const ListLimit = 200

// Enqueuer starts background grading.
type Enqueuer interface {
	Enqueue(job grading.Job) error
}

// AgentDropper releases a session's in-memory chat agent.
type AgentDropper interface {
	Drop(sessionID uuid.UUID)
}

// Handler serves the submissions API.
type Handler struct {
	Store    Store
	Problems problems.Store
	Sessions sessions.Store
	Sandbox  grading.Sandbox // runs the sample test before accepting
	Grading  Enqueuer
	Agents   AgentDropper // optional
}

func (h Handler) Routes(rt *httpx.Router) {
	rt.Handle("POST /api/submissions", h.submit)
	rt.Handle("GET /api/submissions/{id}", h.get)
	rt.Handle("DELETE /api/submissions/{id}", h.delete)
	rt.Handle("GET /api/me/submissions", h.listMine)
}

type submitRequest struct {
	ProblemSlug  string            `json:"problem_slug"`
	Variant      string            `json:"variant"`
	CodeFiles    map[string]string `json:"code_files"`
	SubmissionID string            `json:"submission_id"`
	SessionID    string            `json:"session_id"`
}

// submit validates, runs the problem's sample test (if any) so a broken
// solution fails fast without a grader run, stores the submission, closes
// the session, and enqueues grading. Responds 202 {id, status}.
func (h Handler) submit(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.Require(w, r)
	if !ok {
		return
	}
	var req submitRequest
	if !httpx.Decode(w, r, &req) {
		return
	}
	if req.Variant == "" {
		req.Variant = problems.VariantAsIs
	}
	ctx := r.Context()

	// A reused pending submission isn't a new burn; only count fresh ones.
	if req.SubmissionID == "" {
		n, err := h.Store.CountSince(ctx, u.ID, time.Now().Add(-time.Hour))
		if err != nil {
			httpx.Internal(w, r, err)
			return
		}
		if n >= RateLimitPerHour {
			w.Header().Set("Retry-After", "3600")
			httpx.ErrorWith(w, http.StatusTooManyRequests, "rate_limited",
				fmt.Sprintf("You can submit %d times per hour.", RateLimitPerHour),
				map[string]any{"limit": RateLimitPerHour, "window": "1 hour"})
			return
		}
	}

	// The session's transcript feeds grading and submit tears down its
	// agent, so only the owner may attach it.
	var sessionID *uuid.UUID
	if req.SessionID != "" {
		if id, err := uuid.Parse(req.SessionID); err == nil {
			owned, err := h.Sessions.SessionOwnedBy(ctx, id, u)
			if err != nil {
				httpx.Internal(w, r, err)
				return
			}
			if !owned {
				httpx.NotFound(w, "Session not found.")
				return
			}
			sessionID = &id
		}
	}

	prob, err := h.Problems.Get(ctx, req.ProblemSlug)
	if errors.Is(err, problems.ErrNotFound) {
		httpx.NotFound(w, "Problem not found.")
		return
	}
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	problemID, err := uuid.Parse(prob.ID)
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	starter := prob.Starter.Variant(req.Variant)

	if !h.passesSampleTest(w, r, prob, starter, req.CodeFiles) {
		return
	}

	draft := Draft{UserID: u.ID, ProblemID: problemID, Variant: req.Variant, Code: req.CodeFiles, SessionID: sessionID}
	// Reuse the pending submission created during chat when given;
	// otherwise insert a new one.
	submissionID, parseErr := uuid.Parse(req.SubmissionID)
	if req.SubmissionID != "" && parseErr == nil {
		err = h.Store.Resubmit(ctx, submissionID, draft)
	} else {
		submissionID, err = h.Store.Create(ctx, draft)
	}
	if errors.Is(err, ErrNotFound) {
		httpx.NotFound(w, "Submission not found.")
		return
	}
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}

	// Close the session: it must not be offered for resume, and its agent
	// is no longer needed (the transcript is already in session_messages).
	if sessionID != nil {
		if err := h.Sessions.MarkSubmitted(ctx, *sessionID); err != nil {
			log.Printf("[submit %s] mark session %s submitted: %v", submissionID, *sessionID, err)
		}
		if h.Agents != nil {
			h.Agents.Drop(*sessionID)
		}
	}

	if err := h.Grading.Enqueue(grading.Job{
		SubmissionID:      submissionID,
		ProblemTitle:      prob.Title,
		Difficulty:        prob.Difficulty,
		ReadmeMD:          prob.ReadmeMD,
		Starter:           starter,
		Candidate:         req.CodeFiles,
		HiddenTestFile:    prob.HiddenTestFile,
		HiddenTestContent: prob.HiddenTestContent,
	}); err != nil {
		httpx.Error(w, http.StatusServiceUnavailable, "unavailable", "Grading is restarting. Please submit again in a moment.")
		return
	}
	httpx.JSON(w, http.StatusAccepted, map[string]any{"id": submissionID, "status": "grading"})
}

// passesSampleTest runs the problem's visible sample test. On failure it
// writes a 422 with the diagnostic so the candidate can fix it without
// spending a grader run.
func (h Handler) passesSampleTest(w http.ResponseWriter, r *http.Request, prob *problems.Problem, starter, code map[string]string) bool {
	if prob.SampleTestFilename == nil || prob.SampleTestContent == nil || *prob.SampleTestContent == "" {
		return true
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	res, err := h.Sandbox.RunPytest(ctx, grader.SandboxInput{
		StarterFiles:       starter,
		CandidateFiles:     code,
		HiddenTestFilename: *prob.SampleTestFilename,
		HiddenTestContent:  *prob.SampleTestContent,
		TimeoutSec:         15,
	})
	if err != nil {
		log.Printf("[submit] sample test sandbox: %v", err)
		httpx.ErrorWith(w, http.StatusUnprocessableEntity, "sample_test_unavailable",
			"Couldn't run the sample test. Please try again.", map[string]any{"stage": "sample_test"})
		return false
	}
	if res != nil && res.PassCount < res.Total {
		httpx.ErrorWith(w, http.StatusUnprocessableEntity, "sample_test_failed",
			"Fix the sample test before submitting.", map[string]any{
				"stage":    "sample_test",
				"filename": *prob.SampleTestFilename,
				"passed":   res.PassCount,
				"total":    res.Total,
				"stdout":   res.Stdout,
				"stderr":   res.Stderr,
				"advice":   "fix the sample test before submitting",
			})
		return false
	}
	return true
}

// listMine returns the caller's submissions, newest first.
func (h Handler) listMine(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.Require(w, r)
	if !ok {
		return
	}
	out, err := h.Store.ListMine(r.Context(), u.ID, ListLimit)
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

// get returns one of the caller's submissions. A weak ETag of
// (status, graded_at) lets polling clients get 304 while grading.
func (h Handler) get(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.Require(w, r)
	if !ok {
		return
	}
	id, ok := httpx.PathUUID(w, r, "id")
	if !ok {
		return
	}
	d, err := h.Store.GetMine(r.Context(), id, u.ID)
	if errors.Is(err, ErrNotFound) {
		httpx.NotFound(w, "Submission not found.")
		return
	}
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	var gradedUnix int64
	if d.GradedAt != nil {
		gradedUnix = d.GradedAt.UnixNano()
	}
	etag := fmt.Sprintf(`W/"%s-%d"`, d.Status, gradedUnix)
	w.Header().Set("ETag", etag)
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	httpx.JSON(w, http.StatusOK, d)
}

// delete removes one of the caller's submissions (404 for anyone else's).
func (h Handler) delete(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.Require(w, r)
	if !ok {
		return
	}
	id, ok := httpx.PathUUID(w, r, "id")
	if !ok {
		return
	}
	err := h.Store.DeleteMine(r.Context(), id, u.ID)
	if errors.Is(err, ErrNotFound) {
		httpx.NotFound(w, "Submission not found.")
		return
	}
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
