package problems

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"testing"

	"codritium/backend/internal/auth"
	"codritium/backend/internal/config"
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

func TestSeedTagsAreInVocabulary(t *testing.T) {
	paths, err := config.ResolvePaths()
	if err != nil {
		t.Fatal(err)
	}
	files, _ := filepath.Glob(filepath.Join(paths.ProblemsSeed, "*.json"))
	if len(files) == 0 {
		t.Skip("no seed problems")
	}
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var rec struct {
			Tags []string `json:"tags"`
		}
		if err := json.Unmarshal(raw, &rec); err != nil {
			t.Fatalf("%s: %v", filepath.Base(f), err)
		}
		if len(rec.Tags) == 0 {
			t.Errorf("%s: no tags", filepath.Base(f))
		}
		for _, tag := range rec.Tags {
			if !slices.Contains(Tags, tag) {
				t.Errorf("%s: tag %q is not in problems.Tags", filepath.Base(f), tag)
			}
		}
	}
}

func TestHandler_Search(t *testing.T) {
	pool := testutil.Pool(t)
	h := Handler{Store: Store{Pool: pool}}
	u := testutil.NewUser(t, pool, "user")
	search := func(t *testing.T, as *auth.User, query string) SearchPage {
		t.Helper()
		rr := testutil.Call(t, http.HandlerFunc(h.search), "GET", "/api/problems/search?"+query, nil, as, "")
		testutil.WantStatus(t, rr, http.StatusOK)
		return testutil.Decode[SearchPage](t, rr)
	}

	all := search(t, u, "limit=100")
	if all.Total == 0 {
		t.Skip("no seeded problems")
	}

	t.Run("paging walks the whole catalog once", func(t *testing.T) {
		seen := map[string]bool{}
		offset := 0
		for {
			page := search(t, u, "limit=7&offset="+strconv.Itoa(offset))
			if page.Total != all.Total {
				t.Fatalf("total changed mid-walk: %d vs %d", page.Total, all.Total)
			}
			for _, it := range page.Items {
				if seen[it.Slug] {
					t.Fatalf("%s returned twice", it.Slug)
				}
				seen[it.Slug] = true
			}
			if page.NextOffset == nil {
				break
			}
			offset = *page.NextOffset
		}
		if len(seen) != all.Total {
			t.Fatalf("walked %d problems, total says %d", len(seen), all.Total)
		}
	})

	t.Run("filters", func(t *testing.T) {
		first := all.Items[0]
		tag := ""
		for _, it := range all.Items {
			if len(it.Tags) > 0 {
				tag = it.Tags[0]
				break
			}
		}
		cases := []struct {
			query string
			ok    func(SearchItem) bool
		}{
			{"category=" + first.Category, func(it SearchItem) bool { return it.Category == first.Category }},
			{"difficulty=" + first.Difficulty, func(it SearchItem) bool { return it.Difficulty == first.Difficulty }},
			{"q=" + url.QueryEscape(first.Title), func(it SearchItem) bool { return it.Title == first.Title }},
			{"tag=" + url.QueryEscape(tag), func(it SearchItem) bool { return tag == "" || slices.Contains(it.Tags, tag) }},
			{"status=draft", func(it SearchItem) bool { return it.Status == StatusPublished }},
		}
		for _, c := range cases {
			page := search(t, u, c.query+"&limit=100")
			if len(page.Items) == 0 {
				t.Errorf("%s: no results", c.query)
			}
			for _, it := range page.Items {
				if !c.ok(it) {
					t.Errorf("%s: unexpected row %s", c.query, it.Slug)
				}
			}
		}
		if page := search(t, u, "q="+url.QueryEscape("%")); page.Total != 0 {
			t.Errorf("q=%% matched %d rows; wildcards must be literal", page.Total)
		}
	})

	t.Run("user status", func(t *testing.T) {
		if len(all.Items) < 2 {
			t.Skip("need two problems")
		}
		solved, attempted := all.Items[0].Slug, all.Items[1].Slug
		ctx := context.Background()
		if _, err := pool.Exec(ctx, `
			INSERT INTO submissions (user_id, problem_id, status)
			SELECT $1, id, 'graded' FROM problems WHERE slug = $2`, u.ID, solved); err != nil {
			t.Fatal(err)
		}
		for _, slug := range []string{solved, attempted} {
			if _, err := pool.Exec(ctx, `
				INSERT INTO candidate_sessions (challenge_id, difficulty, user_id) VALUES ($1, 'easy', $2)`,
				slug, u.ID); err != nil {
				t.Fatal(err)
			}
		}
		for status, want := range map[string]string{UserStatusSolved: solved, UserStatusAttempted: attempted} {
			page := search(t, u, "user_status="+status)
			if len(page.Items) != 1 || page.Items[0].Slug != want || page.Items[0].UserStatus != status {
				t.Errorf("user_status=%s: got %+v, want only %s", status, page.Items, want)
			}
		}
		if page := search(t, u, "user_status=todo"); page.Total != all.Total-2 {
			t.Errorf("todo total = %d, want %d", page.Total, all.Total-2)
		}
		if page := search(t, nil, "user_status=solved"); page.Total != 0 {
			t.Errorf("anonymous caller has %d solved problems", page.Total)
		}

		rr := testutil.Call(t, http.HandlerFunc(h.facets), "GET", "/api/problems/facets", nil, u, "")
		testutil.WantStatus(t, rr, http.StatusOK)
		f := testutil.Decode[Facets](t, rr)
		if f.Solved != 1 || f.Total != all.Total {
			t.Errorf("facets solved=%d total=%d, want 1 and %d", f.Solved, f.Total, all.Total)
		}
		sum := 0
		for _, c := range f.Categories {
			sum += c.Count
		}
		if sum != f.Total {
			t.Errorf("category counts sum to %d, total is %d", sum, f.Total)
		}
		for _, c := range f.Tags {
			if page := search(t, u, "tag="+url.QueryEscape(c.Value)); page.Total != c.Count {
				t.Errorf("tag %q: facet says %d, search finds %d", c.Value, c.Count, page.Total)
			}
		}
	})

	t.Run("bad params", func(t *testing.T) {
		for _, query := range []string{"limit=0", "limit=x", "offset=-1", "user_status=done"} {
			rr := testutil.Call(t, http.HandlerFunc(h.search), "GET", "/api/problems/search?"+query, nil, u, "")
			testutil.WantStatus(t, rr, http.StatusBadRequest)
		}
	})
}
