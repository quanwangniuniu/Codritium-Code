package problems

import (
	"net/http"
	"testing"

	"codritium/backend/internal/auth"
	"codritium/backend/internal/platform/testutil"
)

func TestStarterFiles_VariantFallsBackToAsIs(t *testing.T) {
	sf := StarterFiles{
		"a.py": {"as-is": "A", "stripped": "a"},
		"b.py": {"as-is": "B"},
		"c.py": {"stripped": "c"},
	}
	got := sf.Variant(VariantStripped)
	if got["a.py"] != "a" || got["b.py"] != "B" || got["c.py"] != "c" || len(got) != 3 {
		t.Fatalf("stripped=%v", got)
	}
	got = sf.Variant(VariantAsIs)
	if _, ok := got["c.py"]; ok || got["a.py"] != "A" {
		t.Fatalf("as-is=%v", got)
	}
}

func TestVariantAllowed(t *testing.T) {
	cases := []struct {
		policy, variant string
		ok              bool
	}{
		{"both", VariantAsIs, true}, {"both", VariantStripped, true},
		{"as-is-only", VariantStripped, false}, {"stripped-only", VariantAsIs, false},
		{"both", "weird", false},
	}
	for _, c := range cases {
		if got := (&Problem{StripVariant: c.policy}).VariantAllowed(c.variant); got != c.ok {
			t.Errorf("%s/%s: got %v", c.policy, c.variant, got)
		}
	}
}

func TestCanOpen(t *testing.T) {
	admin := &auth.User{Role: "admin", Tier: "standard"}
	std := &auth.User{Role: "user", Tier: "standard"}
	pro := &auth.User{Role: "user", Tier: "pro"}
	premium := &Problem{Status: StatusPublished, Category: CategoryCompanyPremium}
	cases := []struct {
		name string
		u    *auth.User
		p    *Problem
		want Access
	}{
		{"published", std, &Problem{Status: StatusPublished}, AccessOK},
		{"disabled even for admin", admin, &Problem{Status: StatusDisabled}, AccessNotFound},
		{"draft hidden", std, &Problem{Status: StatusDraft}, AccessNotFound},
		{"draft admin", admin, &Problem{Status: StatusDraft}, AccessOK},
		{"premium anon", nil, premium, AccessAuthRequired},
		{"premium standard", std, premium, AccessProRequired},
		{"premium pro", pro, premium, AccessOK},
	}
	for _, c := range cases {
		if got := CanOpen(c.u, c.p); got != c.want {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}

func TestHandler_ListAndGet(t *testing.T) {
	pool := testutil.Pool(t)
	h := Handler{Store: Store{Pool: pool}}
	u := testutil.NewUser(t, pool, "user")

	rr := testutil.Call(t, http.HandlerFunc(h.list), "GET", "/api/problems?status=draft", nil, u, "")
	testutil.WantStatus(t, rr, http.StatusOK)
	list := testutil.Decode[[]Brief](t, rr)
	if len(list) == 0 {
		t.Skip("no seeded problems")
	}
	for _, b := range list {
		if b.Status != StatusPublished {
			t.Fatalf("non-admin saw status %q", b.Status)
		}
	}

	var slug string
	for _, b := range list {
		if !b.RequiresPro {
			slug = b.Slug
			break
		}
	}
	rr = testutil.Call(t, http.HandlerFunc(h.get), "GET", "/api/problems/"+slug, map[string]string{"slug": slug}, u, "")
	testutil.WantStatus(t, rr, http.StatusOK)
	body := testutil.Decode[map[string]any](t, rr)
	if _, leaked := body["hidden_test_content"]; leaked {
		t.Fatal("hidden test leaked")
	}
	if files, _ := body["starter_files"].(map[string]any); len(files) == 0 {
		t.Fatalf("no starter files: %v", body)
	}

	rr = testutil.Call(t, http.HandlerFunc(h.get), "GET", "/api/problems/nope", map[string]string{"slug": "nope-missing"}, u, "")
	testutil.WantStatus(t, rr, http.StatusNotFound)
}
