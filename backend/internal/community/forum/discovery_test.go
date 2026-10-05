package forum

import (
	"net/http"
	"net/url"
	"slices"
	"testing"

	"github.com/google/uuid"

	"codritium/backend/internal/platform/testutil"
)

type feedPage struct {
	Posts   []forumPost
	HasMore bool   `json:"has_more"`
	AsOf    string `json:"as_of"`
}

func TestForum_FeedSnapshotPaging(t *testing.T) {
	pool := testutil.Pool(t)
	deps := Handler{Pool: pool}
	alice := testutil.NewUser(t, pool, "admin") // admin: skips the post rate limit
	tag := uniqueTag()
	for i := 0; i < 3; i++ {
		createForumPost(t, deps, alice, `{"section":"general","title":"Snapshot post `+string(rune('A'+i))+`","body_md":"x","tags":["`+tag+`"]}`)
	}
	get := func(query string) feedPage {
		rr := forumCall(t, http.HandlerFunc(deps.listForumPosts), http.MethodGet, "/api/forum/posts?tag="+tag+"&sort=newest&limit=2"+query, "", nil, "")
		testutil.WantStatus(t, rr, http.StatusOK)
		return decodeForum[feedPage](t, rr)
	}
	first := get("")
	if len(first.Posts) != 2 || !first.HasMore || first.AsOf == "" {
		t.Fatalf("first page: %+v", first)
	}
	// A post written mid-scroll stays out of later pages of the same scroll,
	// so nothing shifts down and the third original post isn't skipped.
	createForumPost(t, deps, alice, `{"section":"general","title":"Late arrival","body_md":"x","tags":["`+tag+`"]}`)
	second := get("&offset=2&as_of=" + url.QueryEscape(first.AsOf))
	if len(second.Posts) != 1 || second.HasMore || second.Posts[0].Title != "Snapshot post A" {
		t.Fatalf("second page: %+v", second.Posts)
	}
	// A fresh scroll sees it.
	if fresh := get(""); fresh.Posts[0].Title != "Late arrival" {
		t.Fatalf("fresh scroll: %+v", fresh.Posts)
	}
	testutil.WantStatus(t, forumCall(t, http.HandlerFunc(deps.listForumPosts), http.MethodGet, "/api/forum/posts?as_of=yesterday", "", nil, ""), http.StatusBadRequest)
}

func TestForum_ProblemFilter(t *testing.T) {
	pool := testutil.Pool(t)
	deps := Handler{Pool: pool}
	alice := testutil.NewUser(t, pool, "user")
	var slug string
	if err := pool.QueryRow(t.Context(), `SELECT slug FROM problems LIMIT 1`).Scan(&slug); err != nil {
		t.Skipf("no problems seeded: %v", err)
	}
	linked := createForumPost(t, deps, alice, `{"section":"general","title":"About the problem","body_md":"x","problem_slug":"`+slug+`"}`)
	createForumPost(t, deps, alice, `{"section":"general","title":"Unrelated post","body_md":"x"}`)
	page := decodeForum[feedPage](t, forumCall(t, http.HandlerFunc(deps.listForumPosts), http.MethodGet,
		"/api/forum/posts?limit=50&problem="+url.QueryEscape(slug), "", nil, ""))
	var ids []uuid.UUID
	for _, p := range page.Posts {
		if p.ProblemSlug == nil || *p.ProblemSlug != slug {
			t.Fatalf("unlinked post in problem filter: %+v", p)
		}
		ids = append(ids, p.ID)
	}
	if !slices.Contains(ids, linked.ID) {
		t.Fatalf("linked post missing: %v", ids)
	}
}

func TestForum_TagsAndRelated(t *testing.T) {
	pool := testutil.Pool(t)
	deps := Handler{Pool: pool}
	alice := testutil.NewUser(t, pool, "admin")
	a, b := uniqueTag(), uniqueTag()
	main := createForumPost(t, deps, alice, `{"section":"career","title":"Main post here","body_md":"x","tags":["`+a+`","`+b+`"]}`)
	both := createForumPost(t, deps, alice, `{"section":"interview","title":"Shares both tags","body_md":"x","tags":["`+a+`","`+b+`"]}`)
	one := createForumPost(t, deps, alice, `{"section":"interview","title":"Shares one tag","body_md":"x","tags":["`+a+`"]}`)

	tags := decodeForum[struct{ Tags []forumTagCount }](t, forumCall(t, http.HandlerFunc(deps.listForumTags), http.MethodGet, "/?q="+a, "", nil, ""))
	if len(tags.Tags) != 1 || tags.Tags[0] != (forumTagCount{Tag: a, Count: 3}) {
		t.Fatalf("tags: %+v", tags.Tags)
	}

	related := decodeForum[struct{ Posts []forumPost }](t, forumCall(t, http.HandlerFunc(deps.listRelatedForumPosts), http.MethodGet, "/", main.ID.String(), nil, ""))
	if len(related.Posts) < 2 || related.Posts[0].ID != both.ID || related.Posts[1].ID != one.ID {
		var titles []string
		for _, p := range related.Posts {
			titles = append(titles, p.Title)
		}
		t.Fatalf("related order: %v", titles)
	}
	for _, p := range related.Posts {
		if p.ID == main.ID {
			t.Fatal("a post is not related to itself")
		}
	}
	empty := decodeForum[struct{ Posts []forumPost }](t, forumCall(t, http.HandlerFunc(deps.listRelatedForumPosts), http.MethodGet, "/", uuid.NewString(), nil, ""))
	if len(empty.Posts) != 0 {
		t.Fatalf("related for missing post: %+v", empty.Posts)
	}
}

func TestForum_MyComments(t *testing.T) {
	pool := testutil.Pool(t)
	deps := Handler{Pool: pool}
	alice := testutil.NewUser(t, pool, "user")
	bob := testutil.NewUser(t, pool, "user")
	p := createForumPost(t, deps, alice, `{"section":"interview","title":"Comment target","body_md":"x"}`)
	for _, body := range []string{`{"body":"first   one"}`, `{"body":"second","is_anonymous":true}`} {
		testutil.WantStatus(t, forumCall(t, http.HandlerFunc(deps.createForumComment), http.MethodPost, "/", p.ID.String(), bob, body), http.StatusCreated)
	}
	testutil.WantStatus(t, forumCall(t, http.HandlerFunc(deps.createForumComment), http.MethodPost, "/", p.ID.String(), alice, `{"body":"not bob's"}`), http.StatusCreated)

	got := decodeForum[struct{ Comments []myForumComment }](t, forumCall(t, http.HandlerFunc(deps.listMyForumComments), http.MethodGet, "/", "", bob, ""))
	if len(got.Comments) != 2 || got.Comments[0].Excerpt != "second" || !got.Comments[0].IsAnonymous ||
		got.Comments[1].Excerpt != "first one" || got.Comments[1].PostTitle != "Comment target" {
		t.Fatalf("my comments: %+v", got.Comments)
	}
	testutil.WantStatus(t, forumCall(t, http.HandlerFunc(deps.listMyForumComments), http.MethodGet, "/", "", nil, ""), http.StatusUnauthorized)
}
