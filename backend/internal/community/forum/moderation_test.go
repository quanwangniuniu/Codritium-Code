package forum

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"codritium/backend/internal/auth"
	"codritium/backend/internal/platform/testutil"
)

type reportQueue struct {
	Items       []reportedItem
	OpenTargets int `json:"open_targets"`
}

func TestForum_ReportsAndQueue(t *testing.T) {
	pool := testutil.Pool(t)
	deps := Handler{Pool: pool}
	alice := testutil.NewUser(t, pool, "user")
	bob := testutil.NewUser(t, pool, "user")
	carol := testutil.NewUser(t, pool, "user")
	admin := testutil.NewUser(t, pool, "admin")
	p := createForumPost(t, deps, alice, `{"section":"compensation","title":"Buy cheap followers","body_md":"spam spam"}`)
	c := decodeForum[forumComment](t, forumCall(t, http.HandlerFunc(deps.createForumComment), http.MethodPost, "/", p.ID.String(), alice,
		`{"body":"rude reply","is_anonymous":true}`))

	report := func(u *auth.User, body string) int {
		return forumCall(t, http.HandlerFunc(deps.createForumReport), http.MethodPost, "/", "", u, body).Code
	}
	postReport := `{"post_id":"` + p.ID.String() + `","reason":"spam","details":"selling followers"}`
	if got := report(bob, postReport); got != http.StatusCreated {
		t.Fatalf("report=%d", got)
	}
	if got := report(bob, postReport); got != http.StatusConflict {
		t.Fatalf("duplicate report=%d want 409", got)
	}
	if got := report(carol, `{"post_id":"`+p.ID.String()+`","reason":"off_topic"}`); got != http.StatusCreated {
		t.Fatalf("second reporter=%d", got)
	}
	if got := report(bob, `{"post_id":"`+p.ID.String()+`","comment_id":"`+c.ID.String()+`","reason":"abuse"}`); got != http.StatusCreated {
		t.Fatalf("comment report=%d", got)
	}
	if got := report(bob, `{"post_id":"`+p.ID.String()+`","reason":"boring"}`); got != http.StatusBadRequest {
		t.Fatalf("bad reason=%d", got)
	}
	if got := report(bob, `{"post_id":"`+uuid.NewString()+`","reason":"spam"}`); got != http.StatusNotFound {
		t.Fatalf("missing target=%d", got)
	}
	if got := report(bob, `{"post_id":"`+p.ID.String()+`","comment_id":"`+uuid.NewString()+`","reason":"spam"}`); got != http.StatusNotFound {
		t.Fatalf("missing comment=%d", got)
	}

	queue := func() reportQueue {
		rr := forumCall(t, http.HandlerFunc(deps.listForumReports), http.MethodGet, "/?limit=50", "", admin, "")
		testutil.WantStatus(t, rr, http.StatusOK)
		return decodeForum[reportQueue](t, rr)
	}
	testutil.WantStatus(t, forumCall(t, http.HandlerFunc(deps.listForumReports), http.MethodGet, "/", "", bob, ""), http.StatusForbidden)
	find := func(q reportQueue, commentID *uuid.UUID) *reportedItem {
		for i, it := range q.Items {
			if it.PostID == p.ID && ((it.CommentID == nil) == (commentID == nil)) {
				return &q.Items[i]
			}
		}
		return nil
	}
	q := queue()
	postItem, commentItem := find(q, nil), find(q, &c.ID)
	if postItem == nil || postItem.Reports != 2 || !slices.Equal(postItem.Reasons, []string{"off_topic", "spam"}) ||
		!slices.Equal(postItem.Details, []string{"selling followers"}) || postItem.Author.ID != alice.ID {
		t.Fatalf("post item: %+v", postItem)
	}
	// Moderators see who wrote anonymous content.
	if commentItem == nil || !commentItem.IsAnonymous || commentItem.Author.ID != alice.ID || commentItem.Excerpt != "rude reply" {
		t.Fatalf("comment item: %+v", commentItem)
	}

	resolve := func(u *auth.User, body string) int {
		return forumCall(t, http.HandlerFunc(deps.resolveForumReports), http.MethodPost, "/", "", u, body).Code
	}
	if got := resolve(bob, `{"post_id":"`+p.ID.String()+`","action":"dismiss"}`); got != http.StatusForbidden {
		t.Fatalf("non-admin resolve=%d", got)
	}
	// Remove the comment and mute its author for 3 days.
	if got := resolve(admin, `{"post_id":"`+p.ID.String()+`","comment_id":"`+c.ID.String()+`","action":"remove","mute_days":3,"reason":"abuse"}`); got != http.StatusOK {
		t.Fatalf("remove=%d", got)
	}
	if post := decodeForum[forumPost](t, forumCall(t, http.HandlerFunc(deps.getForumPost), http.MethodGet, "/", p.ID.String(), admin, "")); post.CommentCount != 0 {
		t.Fatalf("comment_count after removal=%d", post.CommentCount)
	}
	// Dismiss the post's reports: the post stays.
	if got := resolve(admin, `{"post_id":"`+p.ID.String()+`","action":"dismiss"}`); got != http.StatusOK {
		t.Fatalf("dismiss=%d", got)
	}
	testutil.WantStatus(t, forumCall(t, http.HandlerFunc(deps.getForumPost), http.MethodGet, "/", p.ID.String(), bob, ""), http.StatusOK)
	if q := queue(); find(q, nil) != nil || find(q, &c.ID) != nil {
		t.Fatalf("resolved targets still queued: %+v", q.Items)
	}
	var statuses []string
	rows, _ := pool.Query(context.Background(), `SELECT status FROM forum_reports WHERE post_id = $1 ORDER BY status`, p.ID)
	for rows.Next() {
		var s string
		_ = rows.Scan(&s)
		statuses = append(statuses, s)
	}
	rows.Close()
	if !slices.Equal(statuses, []string{"actioned", "dismissed", "dismissed"}) {
		t.Fatalf("statuses=%v", statuses)
	}

	// The muted author can still read but can't write anything.
	rr := forumCall(t, http.HandlerFunc(deps.createForumComment), http.MethodPost, "/", p.ID.String(), alice, `{"body":"again"}`)
	if rr.Code != http.StatusForbidden || !strings.Contains(rr.Body.String(), `"muted"`) {
		t.Fatalf("muted comment: %d %s", rr.Code, rr.Body.String())
	}
	testutil.WantStatus(t, forumCall(t, http.HandlerFunc(deps.createForumPost), http.MethodPost, "/", "", alice, `{"section":"general","title":"Still here","body_md":"x"}`), http.StatusForbidden)
	testutil.WantStatus(t, forumCall(t, http.HandlerFunc(deps.voteForumPost), http.MethodPost, "/", p.ID.String(), alice, `{"value":1}`), http.StatusForbidden)
	testutil.WantStatus(t, forumCall(t, http.HandlerFunc(deps.updateForumPost), http.MethodPut, "/", p.ID.String(), alice, `{"section":"general","title":"Edited title","body_md":"x"}`), http.StatusForbidden)
	testutil.WantStatus(t, forumCall(t, http.HandlerFunc(deps.getForumPost), http.MethodGet, "/", p.ID.String(), alice, ""), http.StatusOK)

	mutes := decodeForum[struct{ Mutes []forumMute }](t, forumCall(t, http.HandlerFunc(deps.listForumMutes), http.MethodGet, "/", "", admin, ""))
	var found *forumMute
	for i := range mutes.Mutes {
		if mutes.Mutes[i].UserID == alice.ID {
			found = &mutes.Mutes[i]
		}
	}
	if found == nil || found.Reason != "abuse" || found.MutedUntil.Before(time.Now().Add(71*time.Hour)) {
		t.Fatalf("mute: %+v", found)
	}
	testutil.WantStatus(t, forumCall(t, http.HandlerFunc(deps.unmuteForumUser), http.MethodDelete, "/", alice.ID.String(), admin, ""), http.StatusNoContent)
	testutil.WantStatus(t, forumCall(t, http.HandlerFunc(deps.createForumComment), http.MethodPost, "/", p.ID.String(), alice, `{"body":"back"}`), http.StatusCreated)
}

func TestForum_MuteByHandle(t *testing.T) {
	pool := testutil.Pool(t)
	deps := Handler{Pool: pool}
	bob := testutil.NewUser(t, pool, "user")
	admin := testutil.NewUser(t, pool, "admin")
	mute := func(u *auth.User, body string) int {
		return forumCall(t, http.HandlerFunc(deps.muteForumUser), http.MethodPost, "/", "", u, body).Code
	}
	if got := mute(bob, `{"handle":"`+admin.Handle+`","days":1}`); got != http.StatusForbidden {
		t.Fatalf("non-admin mute=%d", got)
	}
	if got := mute(admin, `{"handle":"@`+strings.ToUpper(bob.Handle)+`","days":2}`); got != http.StatusNoContent {
		t.Fatalf("mute by handle=%d", got)
	}
	if got := mute(admin, `{"handle":"`+admin.Handle+`","days":1}`); got != http.StatusBadRequest {
		t.Fatalf("muting an admin=%d", got)
	}
	if got := mute(admin, `{"handle":"`+bob.Handle+`","days":0}`); got != http.StatusBadRequest {
		t.Fatalf("zero days=%d", got)
	}
	if got := mute(admin, `{"handle":"no-such-user-here","days":1}`); got != http.StatusNotFound {
		t.Fatalf("unknown handle=%d", got)
	}
}

func TestForum_LockedThread(t *testing.T) {
	pool := testutil.Pool(t)
	deps := Handler{Pool: pool}
	alice := testutil.NewUser(t, pool, "user")
	admin := testutil.NewUser(t, pool, "admin")
	p := createForumPost(t, deps, alice, `{"section":"career","title":"Heated thread","body_md":"x"}`)
	pid := p.ID.String()
	c := decodeForum[forumComment](t, forumCall(t, http.HandlerFunc(deps.createForumComment), http.MethodPost, "/", pid, alice, `{"body":"before"}`))

	lock := func(u *auth.User, on string) int {
		return forumCall(t, http.HandlerFunc(deps.lockForumPost), http.MethodPost, "/", pid, u, `{"locked":`+on+`}`).Code
	}
	if got := lock(alice, "true"); got != http.StatusForbidden {
		t.Fatalf("author lock=%d", got)
	}
	if got := lock(admin, "true"); got != http.StatusOK {
		t.Fatalf("admin lock=%d", got)
	}
	if post := decodeForum[forumPost](t, forumCall(t, http.HandlerFunc(deps.getForumPost), http.MethodGet, "/", pid, alice, "")); !post.IsLocked {
		t.Fatal("post not marked locked")
	}
	rr := forumCall(t, http.HandlerFunc(deps.createForumComment), http.MethodPost, "/", pid, alice, `{"body":"after"}`)
	if rr.Code != http.StatusForbidden || !strings.Contains(rr.Body.String(), "locked") {
		t.Fatalf("comment on locked: %d %s", rr.Code, rr.Body.String())
	}
	testutil.WantStatus(t, forumCall(t, http.HandlerFunc(deps.updateForumComment), http.MethodPut, "/", c.ID.String(), alice, `{"body":"edit"}`), http.StatusNotFound)
	testutil.WantStatus(t, forumCall(t, http.HandlerFunc(deps.createForumComment), http.MethodPost, "/", pid, admin, `{"body":"Locking this, thanks all"}`), http.StatusCreated)
	testutil.WantStatus(t, forumCall(t, http.HandlerFunc(deps.voteForumPost), http.MethodPost, "/", pid, alice, `{"value":1}`), http.StatusOK)
	if got := lock(admin, "false"); got != http.StatusOK {
		t.Fatalf("unlock=%d", got)
	}
	testutil.WantStatus(t, forumCall(t, http.HandlerFunc(deps.createForumComment), http.MethodPost, "/", pid, alice, `{"body":"after unlock"}`), http.StatusCreated)
}

func TestForum_NewAccountRules(t *testing.T) {
	pool := testutil.Pool(t)
	alice := testutil.NewUser(t, pool, "user") // created just now
	post := func(deps Handler, body string) (int, string) {
		rr := forumCall(t, http.HandlerFunc(deps.createForumPost), http.MethodPost, "/", "", alice, body)
		return rr.Code, rr.Body.String()
	}
	waiting := Handler{Pool: pool, NewAccount: NewAccountRules{Wait: time.Hour}}
	if code, body := post(waiting, `{"section":"general","title":"Hello there","body_md":"x"}`); code != http.StatusForbidden || !strings.Contains(body, "account_too_new") {
		t.Fatalf("new account: %d %s", code, body)
	}
	links := Handler{Pool: pool, NewAccount: NewAccountRules{LinkWindow: time.Hour, MaxLinks: 1}}
	if code, body := post(links, `{"section":"general","title":"Links galore","body_md":"https://a.example http://b.example"}`); code != http.StatusForbidden || !strings.Contains(body, "too_many_links") {
		t.Fatalf("links: %d %s", code, body)
	}
	if code, body := post(links, `{"section":"general","title":"One link","body_md":"see https://a.example"}`); code != http.StatusCreated {
		t.Fatalf("one link: %d %s", code, body)
	}
}

func TestForum_Reputation(t *testing.T) {
	pool := testutil.Pool(t)
	deps := Handler{Pool: pool}
	alice := testutil.NewUser(t, pool, "user")
	bob := testutil.NewUser(t, pool, "user")
	named := createForumPost(t, deps, alice, `{"section":"interview","title":"Named post","body_md":"x"}`)
	anon := createForumPost(t, deps, alice, `{"section":"interview","title":"Anon post","body_md":"x","is_anonymous":true}`)
	for _, id := range []uuid.UUID{named.ID, anon.ID} {
		testutil.WantStatus(t, forumCall(t, http.HandlerFunc(deps.voteForumPost), http.MethodPost, "/", id.String(), bob, `{"value":1}`), http.StatusOK)
	}
	got := decodeForum[forumPost](t, forumCall(t, http.HandlerFunc(deps.getForumPost), http.MethodGet, "/", named.ID.String(), bob, ""))
	// Only the named post's upvote counts: anonymous upvotes would unmask.
	if got.Author == nil || got.Author.Reputation != 1 {
		t.Fatalf("reputation: %+v", got.Author)
	}
}
