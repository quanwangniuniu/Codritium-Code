// Package httpx holds the HTTP plumbing every module shares: JSON
// responses, the single error format, bounded body decoding, path
// parameters, and paging.
//
// Error format — every error response is JSON:
//
//	{"error": "<machine_code>", "message": "<human text>", ...extra}
//
// "error" is a stable snake_case code clients branch on; "message" is for
// people and may change. Extra fields carry structured context (e.g.
// challenge_slug on must_complete_problem).
package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"

	"github.com/google/uuid"
)

// MaxBodyBytes caps request bodies unless a handler asks for more.
const MaxBodyBytes = 1 << 20

// JSON writes v with the given status.
func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// Error writes the standard error body.
func Error(w http.ResponseWriter, status int, code, message string) {
	ErrorWith(w, status, code, message, nil)
}

// ErrorWith writes the standard error body plus extra fields.
func ErrorWith(w http.ResponseWriter, status int, code, message string, extra map[string]any) {
	body := make(map[string]any, len(extra)+2)
	for k, v := range extra {
		body[k] = v
	}
	body["error"] = code
	body["message"] = message
	JSON(w, status, body)
}

// Internal logs err with context and answers 500 without leaking details
// (database errors used to be echoed straight to clients).
func Internal(w http.ResponseWriter, r *http.Request, err error) {
	log.Printf("internal error: %s %s: %v", r.Method, r.URL.Path, err)
	Error(w, http.StatusInternalServerError, "internal", "Something went wrong. Please try again.")
}

// Common responses.
func Unauthorized(w http.ResponseWriter) {
	Error(w, http.StatusUnauthorized, "unauthorized", "Sign in to continue.")
}

func Forbidden(w http.ResponseWriter, message string) {
	Error(w, http.StatusForbidden, "forbidden", message)
}

func NotFound(w http.ResponseWriter, message string) {
	Error(w, http.StatusNotFound, "not_found", message)
}

func BadRequest(w http.ResponseWriter, code, message string) {
	Error(w, http.StatusBadRequest, code, message)
}

// DecodeOptions tunes Decode.
type DecodeOptions struct {
	MaxBytes int64 // 0 means MaxBodyBytes
	Strict   bool  // reject unknown fields
}

// Decode reads a JSON body into dst with a size cap. On failure it writes
// a 400 (or 413) and returns false.
func Decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	return DecodeWith(w, r, dst, DecodeOptions{})
}

// DecodeStrict is Decode that also rejects unknown fields.
func DecodeStrict(w http.ResponseWriter, r *http.Request, dst any) bool {
	return DecodeWith(w, r, dst, DecodeOptions{Strict: true})
}

func DecodeWith(w http.ResponseWriter, r *http.Request, dst any, opts DecodeOptions) bool {
	limit := opts.MaxBytes
	if limit <= 0 {
		limit = MaxBodyBytes
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, limit))
	if opts.Strict {
		dec.DisallowUnknownFields()
	}
	err := dec.Decode(dst)
	if err == nil {
		// Reject trailing data such as a second JSON value.
		if dec.Decode(&struct{}{}) != io.EOF {
			err = errors.New("unexpected data after JSON body")
		}
	}
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			Error(w, http.StatusRequestEntityTooLarge, "body_too_large", "Request body is too large.")
			return false
		}
		BadRequest(w, "bad_json", "Request body is not valid JSON for this endpoint.")
		return false
	}
	return true
}

// PathUUID parses a UUID path value. On failure it writes a 404 — a
// malformed id names nothing — and returns false.
func PathUUID(w http.ResponseWriter, r *http.Request, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.PathValue(name))
	if err != nil {
		NotFound(w, "Not found.")
		return uuid.Nil, false
	}
	return id, true
}

// Page is a limit/offset window.
type Page struct {
	Limit  int
	Offset int
}

// ParsePage reads ?limit and ?offset, clamping limit to [1, max] (default
// def) and offset to [0, maxOffset].
func ParsePage(r *http.Request, def, max, maxOffset int) Page {
	p := Page{Limit: def}
	if n, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && n > 0 {
		p.Limit = min(n, max)
	}
	if n, err := strconv.Atoi(r.URL.Query().Get("offset")); err == nil && n > 0 {
		p.Offset = min(n, maxOffset)
	}
	return p
}
