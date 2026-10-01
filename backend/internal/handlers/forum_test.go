package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"codritium/backend/internal/auth"
	"codritium/backend/internal/platform/testutil"
)

func decodeForum[T any](t *testing.T, rr *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rr.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode: %v; status=%d body=%s", err, rr.Code, rr.Body.String())
	}
	return v
}

// uniqueTag keeps each test's feed queries scoped to its own posts.
func uniqueTag() string { return "t" + strings.ReplaceAll(uuid.NewString()[:8], "-", "") }

func createForumPost(t *testing.T, deps ForumDeps, u *auth.User, body string) forumPost {
	t.Helper()
	rr := forumCall(t, CreateForumPost(deps), http.MethodPost, "/api/forum/posts", "", u, body)
	wantStatus(t, rr, http.StatusCreated)
	return decodeForum[forumPost](t, rr)
}

func TestForum_WritesRequireLogin(t *testing.T) {
	deps := ForumDeps{}
	id := uuid.NewString()
	cases := []struct {
		name string
		h    http.HandlerFunc
		id   string
	}{
		{"create post", CreateForumPost(deps), ""},
		{"update post", UpdateForumPost(deps), id},
		{"delete post", DeleteForumPost(deps), id},
		{"vote post", VoteForumPost(deps), id},
		{"pin post", PinForumPost(deps), id},
		{"create comment", CreateForumComment(deps), id},
		{"vote comment", VoteForumComment(deps), id},
		{"delete comment", DeleteForumComment(deps), id},
	}
	for _, c := range cases {
		rr := forumCall(t, c.h, http.MethodPost, "/", c.id, nil, `{}`)
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("%s: status=%d want 401", c.name, rr.Code)
		}
	}
}

func TestForum_CreateValidation(t *testing.T) {
	pool := testutil.Pool(t)
	deps := ForumDeps{Pool: pool}
	alice := newForumUser(t, pool, "user")
	cases := map[string]struct {
		body string
		code string
	}{
		"bad section":        {`{"section":"contest","title":"Hello world","body_md":"x"}`, "invalid_section"},
		"short title":        {`{"section":"career","title":"Hi","body_md":"x"}`, "invalid_title"},
		"empty body":         {`{"section":"career","title":"Hello world","body_md":"   "}`, "invalid_body"},
		"anon outside allow": {`{"section":"career","title":"Hello world","body_md":"x","is_anonymous":true}`, "anonymous_not_allowed"},
		"too many tags":      {`{"section":"career","title":"Hello world","body_md":"x","tags":["a","b","c","d","e","f"]}`, "too_many_tags"},
		"bad tag":            {`{"section":"career","title":"Hello world","body_md":"x","tags":["no/slash"]}`, "invalid_tag"},
		"unknown problem":    {`{"section":"problems","title":"Hello world","body_md":"x","problem_slug":"no-such-problem"}`, "unknown_problem"},
	}
	for name, c := range cases {
		rr := forumCall(t, CreateForumPost(deps), http.MethodPost, "/", "", alice, c.body)
		if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), c.code) {
			t.Errorf("%s: status=%d body=%s want 400 %s", name, rr.Code, rr.Body.String(), c.code)
		}
	}

	// Tags are normalized: trimmed, lowercased, '#'-stripped, deduped.
	p := createForumPost(t, deps, alice,
		`{"section":"career","title":"  Normalize me  ","body_md":"body","tags":[" #Google ","google","System Design"]}`)
	if p.Title != "Normalize me" || strings.Join(p.Tags, ",") != "google,system-design" {
		t.Fatalf("normalized title=%q tags=%v", p.Title, p.Tags)
	}
}

func TestForum_PostLifecycleAndOwnership(t *testing.T) {
	pool := testutil.Pool(t)
	deps := ForumDeps{Pool: pool}
	alice := newForumUser(t, pool, "user")
	bob := newForumUser(t, pool, "user")
	admin := newForumUser(t, pool, "admin")

	p := createForumPost(t, deps, alice,
		`{"section":"interview","title":"Onsite loop recap","body_md":"## Round 1\n**Two sum** variant with `+"`maps`"+`."}`)
	if !p.IsMine || p.Author == nil || p.Author.ID != alice.ID || p.BodyMD == "" {
		t.Fatalf("create response: %+v", p)
	}
	if p.Excerpt != "Round 1 Two sum variant with maps." {
		t.Fatalf("excerpt=%q", p.Excerpt)
	}

	// Views count once per signed-in user; anonymous reads don't count.
	for i := 0; i < 2; i++ {
		forumCall(t, GetForumPost(deps), http.MethodGet, "/", p.ID.String(), bob, "")
	}
	forumCall(t, GetForumPost(deps), http.MethodGet, "/", p.ID.String(), nil, "")
	got := decodeForum[forumPost](t, forumCall(t, GetForumPost(deps), http.MethodGet, "/", p.ID.String(), alice, ""))
	if got.ViewCount != 2 {
		t.Fatalf("view_count=%d want 2 (bob once, alice once)", got.ViewCount)
	}

	edit := `{"section":"interview","title":"Onsite loop recap (edited)","body_md":"new body"}`
	wantStatus(t, forumCall(t, UpdateForumPost(deps), http.MethodPut, "/", p.ID.String(), bob, edit), http.StatusNotFound)
	wantStatus(t, forumCall(t, UpdateForumPost(deps), http.MethodPut, "/", p.ID.String(), admin, edit), http.StatusNotFound)
	updated := decodeForum[forumPost](t, forumCall(t, UpdateForumPost(deps), http.MethodPut, "/", p.ID.String(), alice, edit))
	if updated.Title != "Onsite loop recap (edited)" || !updated.UpdatedAt.After(updated.CreatedAt) {
		t.Fatalf("update: title=%q created=%v updated=%v", updated.Title, updated.CreatedAt, updated.UpdatedAt)
	}

	wantStatus(t, forumCall(t, DeleteForumPost(deps), http.MethodDelete, "/", p.ID.String(), bob, ""), http.StatusNotFound)
	wantStatus(t, forumCall(t, DeleteForumPost(deps), http.MethodDelete, "/", p.ID.String(), admin, ""), http.StatusNoContent)
	wantStatus(t, forumCall(t, GetForumPost(deps), http.MethodGet, "/", p.ID.String(), alice, ""), http.StatusNotFound)
	wantStatus(t, forumCall(t, VoteForumPost(deps), http.MethodPost, "/", p.ID.String(), bob, `{"value":1}`), http.StatusNotFound)
}

func TestForum_AnonymousHidesAuthor(t *testing.T) {
	pool := testutil.Pool(t)
	deps := ForumDeps{Pool: pool}
	alice := newForumUser(t, pool, "user")
	bob := newForumUser(t, pool, "user")
	admin := newForumUser(t, pool, "admin")
	tag := uniqueTag()

	p := createForumPost(t, deps, alice,
		`{"section":"compensation","title":"Offer numbers","body_md":"TC details","is_anonymous":true,"tags":["`+tag+`"]}`)

	asBob := decodeForum[forumPost](t, forumCall(t, GetForumPost(deps), http.MethodGet, "/", p.ID.String(), bob, ""))
	if asBob.Author != nil || asBob.IsMine || !asBob.IsAnonymous {
		t.Fatalf("bob sees author=%+v is_mine=%v", asBob.Author, asBob.IsMine)
	}
	if strings.Contains(forumCall(t, GetForumPost(deps), http.MethodGet, "/", p.ID.String(), bob, "").Body.String(), alice.ID.String()) {
		t.Fatal("anonymous post leaks the author's id to other users")
	}
	asAlice := decodeForum[forumPost](t, forumCall(t, GetForumPost(deps), http.MethodGet, "/", p.ID.String(), alice, ""))
	asAdmin := decodeForum[forumPost](t, forumCall(t, GetForumPost(deps), http.MethodGet, "/", p.ID.String(), admin, ""))
	if asAlice.Author == nil || !asAlice.IsMine || asAdmin.Author == nil || asAdmin.Author.ID != alice.ID {
		t.Fatalf("author/admin should see author: alice=%+v admin=%+v", asAlice.Author, asAdmin.Author)
	}

	feed := forumCall(t, ListForumPosts(deps), http.MethodGet, "/api/forum/posts?tag="+tag, "", bob, "")
	if strings.Contains(feed.Body.String(), alice.ID.String()) || strings.Contains(feed.Body.String(), alice.Handle) {
		t.Fatal("feed leaks the anonymous author")
	}

	// Anonymous comments are allowed on this post and mark the OP.
	c := decodeForum[forumComment](t, forumCall(t, CreateForumComment(deps), http.MethodPost, "/", p.ID.String(), alice,
		`{"body":"Update: negotiated +10%","is_anonymous":true}`))
	list := decodeForum[struct{ Comments []forumComment }](t,
		forumCall(t, ListForumComments(deps), http.MethodGet, "/", p.ID.String(), bob, ""))
	if len(list.Comments) != 1 || list.Comments[0].ID != c.ID || list.Comments[0].Author != nil || !list.Comments[0].IsOP {
		t.Fatalf("comment as bob: %+v", list.Comments)
	}

	// ...but not on a section that doesn't allow anonymity.
	career := createForumPost(t, deps, alice, `{"section":"career","title":"Career question","body_md":"x"}`)
	rr := forumCall(t, CreateForumComment(deps), http.MethodPost, "/", career.ID.String(), bob, `{"body":"hi","is_anonymous":true}`)
	if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), "anonymous_not_allowed") {
		t.Fatalf("anon comment on career: %d %s", rr.Code, rr.Body.String())
	}
}

func TestForum_VotesUpdateCounters(t *testing.T) {
	pool := testutil.Pool(t)
	deps := ForumDeps{Pool: pool}
	alice := newForumUser(t, pool, "user")
	bob := newForumUser(t, pool, "user")
	p := createForumPost(t, deps, alice, `{"section":"feedback","title":"Dark mode request","body_md":"please"}`)

	type voteResp struct {
		Upvotes, Downvotes, Score int
		MyVote                    int `json:"my_vote"`
	}
	vote := func(v string) voteResp {
		rr := forumCall(t, VoteForumPost(deps), http.MethodPost, "/", p.ID.String(), bob, `{"value":`+v+`}`)
		wantStatus(t, rr, http.StatusOK)
		return decodeForum[voteResp](t, rr)
	}
	if r := vote("1"); r.Upvotes != 1 || r.Score != 1 || r.MyVote != 1 {
		t.Fatalf("upvote: %+v", r)
	}
	if r := vote("1"); r.Upvotes != 1 {
		t.Fatalf("repeat upvote must not double count: %+v", r)
	}
	if r := vote("-1"); r.Upvotes != 0 || r.Downvotes != 1 || r.Score != -1 {
		t.Fatalf("flip to downvote: %+v", r)
	}
	if r := vote("0"); r.Upvotes != 0 || r.Downvotes != 0 || r.MyVote != 0 {
		t.Fatalf("clear vote: %+v", r)
	}
	wantStatus(t, forumCall(t, VoteForumPost(deps), http.MethodPost, "/", p.ID.String(), bob, `{"value":2}`), http.StatusBadRequest)
	wantStatus(t, forumCall(t, VoteForumPost(deps), http.MethodPost, "/", uuid.NewString(), bob, `{"value":1}`), http.StatusNotFound)

	vote("1")
	asBob := decodeForum[forumPost](t, forumCall(t, GetForumPost(deps), http.MethodGet, "/", p.ID.String(), bob, ""))
	if asBob.MyVote != 1 || asBob.Score != 1 {
		t.Fatalf("post reflects vote: my_vote=%d score=%d", asBob.MyVote, asBob.Score)
	}
}

func TestForum_CommentsAndReplies(t *testing.T) {
	pool := testutil.Pool(t)
	deps := ForumDeps{Pool: pool}
	alice := newForumUser(t, pool, "user")
	bob := newForumUser(t, pool, "user")
	p := createForumPost(t, deps, alice, `{"section":"career","title":"Switching to backend","body_md":"advice?"}`)
	pid := p.ID.String()

	comment := func(u *auth.User, body string) *httptest.ResponseRecorder {
		return forumCall(t, CreateForumComment(deps), http.MethodPost, "/", pid, u, body)
	}
	top := decodeForum[forumComment](t, comment(bob, `{"body":"Learn SQL first"}`))
	second := decodeForum[forumComment](t, comment(alice, `{"body":"Thanks all"}`))
	reply := decodeForum[forumComment](t, comment(alice, `{"body":"Good call","parent_id":"`+top.ID.String()+`"}`))
	if !reply.IsOP || reply.ParentID == nil || *reply.ParentID != top.ID {
		t.Fatalf("reply: %+v", reply)
	}

	// Threads are one level deep; replies to replies and cross-post parents fail.
	wantStatus(t, comment(bob, `{"body":"nested","parent_id":"`+reply.ID.String()+`"}`), http.StatusBadRequest)
	other := createForumPost(t, deps, bob, `{"section":"career","title":"Another post","body_md":"x"}`)
	wantStatus(t, forumCall(t, CreateForumComment(deps), http.MethodPost, "/", other.ID.String(), bob,
		`{"body":"x","parent_id":"`+top.ID.String()+`"}`), http.StatusBadRequest)
	wantStatus(t, comment(bob, `{"body":"   "}`), http.StatusBadRequest)

	// Best sort puts the upvoted comment first; replies are nested under it.
	wantStatus(t, forumCall(t, VoteForumComment(deps), http.MethodPost, "/", second.ID.String(), bob, `{"value":1}`), http.StatusOK)
	list := decodeForum[struct{ Comments []forumComment }](t,
		forumCall(t, ListForumComments(deps), http.MethodGet, "/?sort=best", pid, bob, ""))
	if len(list.Comments) != 2 || list.Comments[0].ID != second.ID || len(list.Comments[1].Replies) != 1 {
		t.Fatalf("best order/replies: %+v", list.Comments)
	}
	newest := decodeForum[struct{ Comments []forumComment }](t,
		forumCall(t, ListForumComments(deps), http.MethodGet, "/?sort=newest", pid, bob, ""))
	if newest.Comments[0].ID != second.ID || newest.Comments[1].ID != top.ID {
		t.Fatalf("newest order: %+v", newest.Comments)
	}

	count := func() int {
		return decodeForum[forumPost](t, forumCall(t, GetForumPost(deps), http.MethodGet, "/", pid, alice, "")).CommentCount
	}
	if n := count(); n != 3 {
		t.Fatalf("comment_count=%d want 3", n)
	}
	// Only the author (or an admin) can delete; deleting a top-level comment
	// takes its replies with it.
	wantStatus(t, forumCall(t, DeleteForumComment(deps), http.MethodDelete, "/", top.ID.String(), alice, ""), http.StatusNotFound)
	wantStatus(t, forumCall(t, DeleteForumComment(deps), http.MethodDelete, "/", top.ID.String(), bob, ""), http.StatusNoContent)
	if n := count(); n != 1 {
		t.Fatalf("comment_count after delete=%d want 1", n)
	}
	after := decodeForum[struct{ Comments []forumComment }](t,
		forumCall(t, ListForumComments(deps), http.MethodGet, "/", pid, bob, ""))
	if len(after.Comments) != 1 || after.Comments[0].ID != second.ID {
		t.Fatalf("after delete: %+v", after.Comments)
	}
}

func TestForum_FeedFiltersSortAndSearch(t *testing.T) {
	pool := testutil.Pool(t)
	deps := ForumDeps{Pool: pool}
	alice := newForumUser(t, pool, "user")
	bob := newForumUser(t, pool, "user")
	tag := uniqueTag()

	mk := func(section, title string) forumPost {
		return createForumPost(t, deps, alice,
			`{"section":"`+section+`","title":"`+title+`","body_md":"body text","tags":["`+tag+`"]}`)
	}
	a := mk("career", "Resume review thread")
	b := mk("interview", "Graph questions at onsite")
	c := mk("career", "Negotiation tactics")
	forumCall(t, VoteForumPost(deps), http.MethodPost, "/", b.ID.String(), bob, `{"value":1}`)

	type feed struct {
		Posts   []forumPost
		HasMore bool `json:"has_more"`
	}
	get := func(q string) feed {
		rr := forumCall(t, ListForumPosts(deps), http.MethodGet, "/api/forum/posts?tag="+tag+q, "", bob, "")
		wantStatus(t, rr, http.StatusOK)
		return decodeForum[feed](t, rr)
	}
	ids := func(f feed) []uuid.UUID {
		out := make([]uuid.UUID, len(f.Posts))
		for i, p := range f.Posts {
			out[i] = p.ID
		}
		return out
	}

	if f := get("&sort=newest"); len(f.Posts) != 3 || f.Posts[0].ID != c.ID || f.Posts[2].ID != a.ID {
		t.Fatalf("newest: %v", ids(f))
	}
	if f := get("&sort=votes"); f.Posts[0].ID != b.ID {
		t.Fatalf("votes: %v", ids(f))
	}
	if f := get("&section=career&sort=newest"); len(f.Posts) != 2 || f.Posts[0].ID != c.ID {
		t.Fatalf("section: %v", ids(f))
	}
	if f := get("&q=graph"); len(f.Posts) != 1 || f.Posts[0].ID != b.ID {
		t.Fatalf("search stemmed word: %v", ids(f))
	}
	if f := get("&q=negotia"); len(f.Posts) != 1 || f.Posts[0].ID != c.ID {
		t.Fatalf("search partial title: %v", ids(f))
	}
	if f := get("&q=100%25_off"); len(f.Posts) != 0 {
		t.Fatalf("LIKE wildcards must be escaped: %v", ids(f))
	}
	if f := get("&sort=newest&limit=2"); len(f.Posts) != 2 || !f.HasMore {
		t.Fatalf("page 1: %d has_more=%v", len(f.Posts), f.HasMore)
	}
	if f := get("&sort=newest&limit=2&offset=2"); len(f.Posts) != 1 || f.HasMore || f.Posts[0].ID != a.ID {
		t.Fatalf("page 2: %v has_more=%v", ids(f), f.HasMore)
	}
	wantStatus(t, forumCall(t, ListForumPosts(deps), http.MethodGet, "/api/forum/posts?sort=bogus", "", nil, ""), http.StatusBadRequest)
	wantStatus(t, forumCall(t, ListForumPosts(deps), http.MethodGet, "/api/forum/posts?section=bogus", "", nil, ""), http.StatusBadRequest)

	// Signed-out visitors can read the feed.
	rr := forumCall(t, ListForumPosts(deps), http.MethodGet, "/api/forum/posts?tag="+tag, "", nil, "")
	wantStatus(t, rr, http.StatusOK)
	if f := decodeForum[feed](t, rr); len(f.Posts) != 3 || f.Posts[0].MyVote != 0 {
		t.Fatalf("signed-out feed: %v", ids(f))
	}
}

func TestForum_PinIsAdminOnly(t *testing.T) {
	pool := testutil.Pool(t)
	deps := ForumDeps{Pool: pool}
	alice := newForumUser(t, pool, "user")
	admin := newForumUser(t, pool, "admin")
	p := createForumPost(t, deps, admin, `{"section":"feedback","title":"Welcome to the forum","body_md":"rules"}`)
	if p.Author == nil || !p.Author.Verified {
		t.Fatalf("admin post should be verified: %+v", p.Author)
	}

	wantStatus(t, forumCall(t, PinForumPost(deps), http.MethodPost, "/", p.ID.String(), alice, `{"pinned":true}`), http.StatusForbidden)
	wantStatus(t, forumCall(t, PinForumPost(deps), http.MethodPost, "/", p.ID.String(), admin, `{"pinned":true}`), http.StatusOK)

	pinned := decodeForum[struct{ Posts []forumPost }](t,
		forumCall(t, ListPinnedForumPosts(deps), http.MethodGet, "/", "", nil, ""))
	found := false
	for _, pp := range pinned.Posts {
		found = found || pp.ID == p.ID
	}
	if !found {
		t.Fatalf("pinned list missing post: %+v", pinned.Posts)
	}
	wantStatus(t, forumCall(t, PinForumPost(deps), http.MethodPost, "/", p.ID.String(), admin, `{"pinned":false}`), http.StatusOK)
}

func TestForumExcerpt(t *testing.T) {
	cases := map[string]string{
		"# Title\n\nSome **bold** and `code` [link](http://x) text": "Title Some bold and code link text",
		"Before\n```go\nfunc main() {}\n```\nAfter":                 "Before After",
		"- one\n- two\n1. three\n> quoted":                          "one two three quoted",
	}
	for in, want := range cases {
		if got := forumExcerpt(in); got != want {
			t.Errorf("forumExcerpt(%q)=%q want %q", in, got, want)
		}
	}
	long := strings.Repeat("word ", 100)
	if got := forumExcerpt(long); !strings.HasSuffix(got, "…") || len([]rune(got)) > forumExcerptLen+1 {
		t.Errorf("long excerpt not truncated: %d runes", len([]rune(got)))
	}
}

func TestForum_TrendingOnlyReadPosts(t *testing.T) {
	pool := testutil.Pool(t)
	deps := ForumDeps{Pool: pool}
	alice := newForumUser(t, pool, "user")
	bob := newForumUser(t, pool, "user")
	unread := createForumPost(t, deps, alice, `{"section":"career","title":"Nobody read this yet","body_md":"x"}`)
	read := createForumPost(t, deps, alice, `{"section":"career","title":"Bob read this one","body_md":"x","tags":["resume"]}`)
	forumCall(t, GetForumPost(deps), http.MethodGet, "/", read.ID.String(), bob, "")

	list := decodeForum[struct{ Posts []forumTrendingItem }](t,
		forumCall(t, ListTrendingForumPosts(deps), http.MethodGet, "/", "", nil, ""))
	var sawRead bool
	for _, it := range list.Posts {
		if it.ID == unread.ID {
			t.Fatal("unviewed post must not trend")
		}
		if it.ID == read.ID {
			sawRead = it.ViewCount == 1 && it.Tag == "resume"
		}
	}
	if !sawRead {
		t.Fatalf("viewed post missing from trending: %+v", list.Posts)
	}
}
