package forum

import (
	"context"
	"net/http"
	"slices"
	"testing"

	"github.com/google/uuid"

	"codritium/backend/internal/auth"
	"codritium/backend/internal/platform/testutil"
)

// inbox returns u's notifications as "kind" strings, oldest first.
func inbox(t *testing.T, deps Handler, u *auth.User) []string {
	t.Helper()
	rows, err := deps.Pool.Query(context.Background(),
		`SELECT kind FROM notifications WHERE user_id = $1 ORDER BY created_at, id`, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			t.Fatal(err)
		}
		out = append(out, k)
	}
	return out
}

func TestMentionedHandles(t *testing.T) {
	cases := map[string][]string{
		"hey @Alice and @bob.":                  {"alice", "bob"},
		"@alice @ALICE twice":                   {"alice"},
		"mail me at a@b.com":                    nil,
		"`@code` and\n```\n@fenced\n```\n@real": {"real"},
		"(@paren) @trail- end":                  {"paren", "trail"},
	}
	for in, want := range cases {
		if got := mentionedHandles(in); !slices.Equal(got, want) {
			t.Errorf("mentionedHandles(%q)=%v want %v", in, got, want)
		}
	}
}

func TestForum_CommentNotifications(t *testing.T) {
	pool := testutil.Pool(t)
	deps := Handler{Pool: pool}
	alice := testutil.NewUser(t, pool, "user") // post author
	bob := testutil.NewUser(t, pool, "user")   // top-level commenter
	carol := testutil.NewUser(t, pool, "user") // replier
	dave := testutil.NewUser(t, pool, "user")  // follower
	erin := testutil.NewUser(t, pool, "user")  // mentioned

	p := createForumPost(t, deps, alice, `{"section":"interview","title":"Notify me please","body_md":"x"}`)
	pid := p.ID.String()
	if !p.IsFollowing {
		t.Fatal("author should follow their own post")
	}
	testutil.WantStatus(t, forumCall(t, http.HandlerFunc(deps.followForumPost), http.MethodPost, "/", pid, dave, `{"following":true}`), http.StatusOK)

	comment := func(u *auth.User, body string) forumComment {
		rr := forumCall(t, http.HandlerFunc(deps.createForumComment), http.MethodPost, "/", pid, u, body)
		testutil.WantStatus(t, rr, http.StatusCreated)
		return decodeForum[forumComment](t, rr)
	}
	top := comment(bob, `{"body":"Great post"}`)
	comment(carol, `{"body":"Agreed @`+erin.Handle+` @`+carol.Handle+`","parent_id":"`+top.ID.String()+`","is_anonymous":true}`)
	comment(alice, `{"body":"thanks"}`) // author's own comment: alice hears nothing new

	want := map[*auth.User][]string{
		alice: {"post_reply", "post_reply"}, // bob's comment, carol's reply; not her own comment
		bob:   {"comment_reply"},            // carol's reply; he doesn't follow, so not alice's comment
		carol: nil,                          // never about her own comment or self-mention
		dave:  {"post_comment", "post_comment", "post_comment"},
		erin:  {"mention"},
	}
	for u, w := range want {
		if got := inbox(t, deps, u); !slices.Equal(got, w) {
			t.Errorf("%s inbox=%v want %v", u.Handle, got, w)
		}
	}

	// Anonymous replies don't reveal who wrote them.
	var actorAnon bool
	if err := pool.QueryRow(context.Background(),
		`SELECT actor_anonymous FROM notifications WHERE user_id = $1 AND kind = 'mention'`, erin.ID).Scan(&actorAnon); err != nil || !actorAnon {
		t.Fatalf("mention from anonymous reply: anon=%v err=%v", actorAnon, err)
	}

	// Unfollowing stops post_comment notifications.
	testutil.WantStatus(t, forumCall(t, http.HandlerFunc(deps.followForumPost), http.MethodPost, "/", pid, dave, `{"following":false}`), http.StatusOK)
	comment(bob, `{"body":"one more"}`)
	if got := inbox(t, deps, dave); len(got) != 3 {
		t.Fatalf("dave after unfollow=%v", got)
	}
}

func TestForum_PostMentionAndMilestone(t *testing.T) {
	pool := testutil.Pool(t)
	deps := Handler{Pool: pool}
	alice := testutil.NewUser(t, pool, "user")
	bob := testutil.NewUser(t, pool, "user")
	p := createForumPost(t, deps, alice, `{"section":"general","title":"Shout out","body_md":"thanks @`+bob.Handle+` and @nobody-here"}`)
	if got := inbox(t, deps, bob); !slices.Equal(got, []string{"mention"}) {
		t.Fatalf("bob=%v", got)
	}

	// Simulate a post just under the first milestone; the upvote that
	// reaches it notifies once, and re-crossing it doesn't repeat.
	if _, err := pool.Exec(context.Background(), `UPDATE forum_posts SET upvotes = 9 WHERE id = $1`, p.ID); err != nil {
		t.Fatal(err)
	}
	vote := func(body string) {
		testutil.WantStatus(t, forumCall(t, http.HandlerFunc(deps.voteForumPost), http.MethodPost, "/", p.ID.String(), bob, body), http.StatusOK)
	}
	vote(`{"value":1}`)
	vote(`{"value":0}`)
	vote(`{"value":1}`)
	if got := inbox(t, deps, alice); !slices.Equal(got, []string{"post_milestone"}) {
		t.Fatalf("alice=%v", got)
	}
}

func TestForum_Bookmarks(t *testing.T) {
	pool := testutil.Pool(t)
	deps := Handler{Pool: pool}
	alice := testutil.NewUser(t, pool, "user")
	a := createForumPost(t, deps, alice, `{"section":"career","title":"First saved post","body_md":"x"}`)
	b := createForumPost(t, deps, alice, `{"section":"career","title":"Second saved post","body_md":"x"}`)
	mark := func(id uuid.UUID, on string) {
		testutil.WantStatus(t, forumCall(t, http.HandlerFunc(deps.bookmarkForumPost), http.MethodPost, "/", id.String(), alice, `{"bookmarked":`+on+`}`), http.StatusOK)
	}
	mark(a.ID, "true")
	mark(b.ID, "true")
	mark(b.ID, "true") // idempotent
	list := func() []uuid.UUID {
		page := decodeForum[struct{ Posts []forumPost }](t,
			forumCall(t, http.HandlerFunc(deps.listForumBookmarks), http.MethodGet, "/", "", alice, ""))
		var ids []uuid.UUID
		for _, p := range page.Posts {
			if !p.IsBookmarked {
				t.Errorf("listed post not marked bookmarked: %s", p.ID)
			}
			ids = append(ids, p.ID)
		}
		return ids
	}
	if got := list(); !slices.Equal(got, []uuid.UUID{b.ID, a.ID}) {
		t.Fatalf("bookmarks=%v", got)
	}
	mark(b.ID, "false")
	if got := list(); !slices.Equal(got, []uuid.UUID{a.ID}) {
		t.Fatalf("after unsave=%v", got)
	}
	testutil.WantStatus(t, forumCall(t, http.HandlerFunc(deps.bookmarkForumPost), http.MethodPost, "/", uuid.NewString(), alice, `{"bookmarked":true}`), http.StatusNotFound)
	testutil.WantStatus(t, forumCall(t, http.HandlerFunc(deps.bookmarkForumPost), http.MethodPost, "/", a.ID.String(), alice, `{}`), http.StatusBadRequest)
	testutil.WantStatus(t, forumCall(t, http.HandlerFunc(deps.listForumBookmarks), http.MethodGet, "/", "", nil, ""), http.StatusUnauthorized)
}

func TestForum_MentionSuggestions(t *testing.T) {
	pool := testutil.Pool(t)
	deps := Handler{Pool: pool}
	alice := testutil.NewUser(t, pool, "user")
	bob := testutil.NewUser(t, pool, "user")
	ghost := testutil.NewUser(t, pool, "user")
	p := createForumPost(t, deps, alice, `{"section":"interview","title":"Who is here","body_md":"x"}`)
	forumCall(t, http.HandlerFunc(deps.createForumComment), http.MethodPost, "/", p.ID.String(), bob, `{"body":"me"}`)
	forumCall(t, http.HandlerFunc(deps.createForumComment), http.MethodPost, "/", p.ID.String(), ghost, `{"body":"anon","is_anonymous":true}`)

	suggest := func(q string) []string {
		rr := forumCall(t, http.HandlerFunc(deps.suggestMentions), http.MethodGet, "/?post_id="+p.ID.String()+"&q="+q, "", alice, "")
		testutil.WantStatus(t, rr, http.StatusOK)
		var out []string
		for _, u := range decodeForum[struct{ Users []mentionSuggestion }](t, rr).Users {
			out = append(out, u.Handle)
		}
		return out
	}
	// Empty query lists only named thread participants, not the asker and
	// not anonymous commenters.
	if got := suggest(""); !slices.Equal(got, []string{bob.Handle}) {
		t.Fatalf("empty q=%v", got)
	}
	if got := suggest(ghost.Handle[:10]); !slices.Contains(got, ghost.Handle) {
		t.Fatalf("prefix q missing ghost: %v", got)
	}
	testutil.WantStatus(t, forumCall(t, http.HandlerFunc(deps.suggestMentions), http.MethodGet, "/?q=a", "", nil, ""), http.StatusUnauthorized)
}
