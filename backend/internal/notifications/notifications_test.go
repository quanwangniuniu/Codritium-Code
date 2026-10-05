package notifications

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"codritium/backend/internal/platform/testutil"
)

type listResp struct {
	Notifications []notification `json:"notifications"`
	HasMore       bool           `json:"has_more"`
}

func TestNotifications_ListCountAndRead(t *testing.T) {
	pool := testutil.Pool(t)
	h := Handler{Pool: pool}
	ctx := context.Background()
	alice := testutil.NewUser(t, pool, "user")
	bob := testutil.NewUser(t, pool, "user")

	var postID, commentID, anonCommentID uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO forum_posts (user_id, section, title, body_md)
		VALUES ($1, 'general', 'A post', 'x') RETURNING id`, alice.ID).Scan(&postID); err != nil {
		t.Fatal(err)
	}
	for _, id := range []*uuid.UUID{&commentID, &anonCommentID} {
		if err := pool.QueryRow(ctx, `INSERT INTO forum_comments (post_id, user_id, body)
			VALUES ($1, $2, 'hello   there') RETURNING id`, postID, bob.ID).Scan(id); err != nil {
			t.Fatal(err)
		}
	}
	if err := Insert(ctx, pool, []New{
		{UserID: alice.ID, Kind: PostReply, ActorID: &bob.ID, PostID: postID, CommentID: &commentID},
		{UserID: alice.ID, Kind: Mention, ActorID: &bob.ID, ActorAnonymous: true, PostID: postID, CommentID: &anonCommentID},
	}); err != nil {
		t.Fatal(err)
	}
	if err := InsertMilestone(ctx, pool, alice.ID, postID, 10); err != nil {
		t.Fatal(err)
	}
	if err := InsertMilestone(ctx, pool, alice.ID, postID, 10); err != nil {
		t.Fatal(err)
	}

	list := func(q string) listResp {
		rr := testutil.Call(t, http.HandlerFunc(h.list), http.MethodGet, "/"+q, nil, alice, "")
		testutil.WantStatus(t, rr, http.StatusOK)
		var v listResp
		if err := json.Unmarshal(rr.Body.Bytes(), &v); err != nil {
			t.Fatal(err)
		}
		return v
	}
	count := func() int {
		rr := testutil.Call(t, http.HandlerFunc(h.unreadCount), http.MethodGet, "/", nil, alice, "")
		testutil.WantStatus(t, rr, http.StatusOK)
		var v struct{ Count int }
		_ = json.Unmarshal(rr.Body.Bytes(), &v)
		return v.Count
	}

	got := list("")
	if len(got.Notifications) != 3 || count() != 3 {
		t.Fatalf("list=%+v count=%d", got.Notifications, count())
	}
	byKind := map[string]notification{}
	for _, n := range got.Notifications {
		byKind[n.Kind] = n
	}
	if n := byKind[PostReply]; n.Actor == nil || n.Actor.ID != bob.ID || n.Excerpt != "hello there" || n.PostTitle != "A post" {
		t.Fatalf("post_reply: %+v", n)
	}
	if n := byKind[Mention]; n.Actor != nil {
		t.Fatalf("anonymous actor leaked: %+v", n.Actor)
	}
	if n := byKind[PostMilestone]; n.Milestone == nil || *n.Milestone != 10 || n.Actor != nil {
		t.Fatalf("milestone: %+v", n)
	}

	// Marking one read, ignoring other users' ids.
	other := testutil.Call(t, http.HandlerFunc(h.markRead), http.MethodPost, "/", nil, bob,
		`{"ids":["`+byKind[PostReply].ID.String()+`"]}`)
	testutil.WantStatus(t, other, http.StatusNoContent)
	if count() != 3 {
		t.Fatal("another user marked alice's notification read")
	}
	testutil.WantStatus(t, testutil.Call(t, http.HandlerFunc(h.markRead), http.MethodPost, "/", nil, alice,
		`{"ids":["`+byKind[PostReply].ID.String()+`"]}`), http.StatusNoContent)
	if count() != 2 || len(list("?unread=1").Notifications) != 2 {
		t.Fatalf("after one read: count=%d", count())
	}

	// A deleted comment hides its notification.
	if _, err := pool.Exec(ctx, `UPDATE forum_comments SET deleted_at = now() WHERE id = $1`, anonCommentID); err != nil {
		t.Fatal(err)
	}
	if count() != 1 {
		t.Fatalf("deleted comment still counted: %d", count())
	}

	testutil.WantStatus(t, testutil.Call(t, http.HandlerFunc(h.markRead), http.MethodPost, "/", nil, alice, `{"all":true}`), http.StatusNoContent)
	if count() != 0 {
		t.Fatalf("after mark all: %d", count())
	}
	testutil.WantStatus(t, testutil.Call(t, http.HandlerFunc(h.markRead), http.MethodPost, "/", nil, alice, `{}`), http.StatusBadRequest)
	testutil.WantStatus(t, testutil.Call(t, http.HandlerFunc(h.list), http.MethodGet, "/", nil, nil, ""), http.StatusUnauthorized)
}
