package httpx

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func body(t *testing.T, rr *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &m); err != nil {
		t.Fatalf("not JSON: %q", rr.Body.String())
	}
	return m
}

func TestErrorWith_Shape(t *testing.T) {
	rr := httptest.NewRecorder()
	ErrorWith(rr, http.StatusForbidden, "must_complete_problem", "Finish it first.", map[string]any{"challenge_slug": "x", "error": "spoof"})
	m := body(t, rr)
	if rr.Code != 403 || m["error"] != "must_complete_problem" || m["message"] != "Finish it first." || m["challenge_slug"] != "x" {
		t.Fatalf("got %d %v", rr.Code, m)
	}
}

func TestInternal_DoesNotLeak(t *testing.T) {
	rr := httptest.NewRecorder()
	Internal(rr, httptest.NewRequest("GET", "/x", nil), errors.New(`pq: relation "users" does not exist`))
	if rr.Code != 500 || strings.Contains(rr.Body.String(), "relation") {
		t.Fatalf("leaked: %d %s", rr.Code, rr.Body.String())
	}
}

func TestDecode(t *testing.T) {
	type req struct {
		Name string `json:"name"`
	}
	cases := []struct {
		name   string
		body   string
		strict bool
		max    int64
		ok     bool
		status int
	}{
		{"valid", `{"name":"a"}`, false, 0, true, 0},
		{"unknown allowed", `{"name":"a","x":1}`, false, 0, true, 0},
		{"unknown strict", `{"name":"a","x":1}`, true, 0, false, 400},
		{"malformed", `{nope`, false, 0, false, 400},
		{"trailing", `{"name":"a"}{"name":"b"}`, false, 0, false, 400},
		{"too big", `{"name":"` + strings.Repeat("a", 100) + `"}`, false, 32, false, 413},
	}
	for _, c := range cases {
		rr := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/", strings.NewReader(c.body))
		var dst req
		ok := DecodeWith(rr, r, &dst, DecodeOptions{Strict: c.strict, MaxBytes: c.max})
		if ok != c.ok || (!ok && rr.Code != c.status) {
			t.Errorf("%s: ok=%v status=%d", c.name, ok, rr.Code)
		}
	}
}

func TestPathUUIDAndPage(t *testing.T) {
	r := httptest.NewRequest("GET", "/?limit=500&offset=-3", nil)
	r.SetPathValue("id", "not-a-uuid")
	rr := httptest.NewRecorder()
	if _, ok := PathUUID(rr, r, "id"); ok || rr.Code != 404 {
		t.Fatalf("bad uuid accepted: %d", rr.Code)
	}
	if p := ParsePage(r, 20, 50, 2000); p.Limit != 50 || p.Offset != 0 {
		t.Fatalf("page=%+v", p)
	}
}
