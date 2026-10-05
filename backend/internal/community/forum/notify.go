package forum

import (
	"context"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"codritium/backend/internal/notifications"
)

// db is what notification fan-out needs: a pool or a transaction.
type db interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

const maxMentions = 10

// forumMilestones are the upvote scores that notify a post's author.
var forumMilestones = []int{10, 50, 100, 500, 1000}

var (
	// A handle after "@" at the start of the text or after a non-word
	// character, so emails (a@b.com) don't count.
	mentionPattern = regexp.MustCompile(`(?:^|[^\w@])@([A-Za-z0-9][A-Za-z0-9_.-]{0,38})`)
	mdCodeFence    = regexp.MustCompile("(?s)```.*?(```|$)")
	mdCodeSpan     = regexp.MustCompile("`[^`\n]*`")
)

// mentionedHandles returns the distinct lowercase handles @mentioned in a
// markdown body, ignoring code, capped at maxMentions.
func mentionedHandles(md string) []string {
	s := mdCodeSpan.ReplaceAllString(mdCodeFence.ReplaceAllString(md, " "), " ")
	var out []string
	seen := map[string]bool{}
	for _, m := range mentionPattern.FindAllStringSubmatch(s, -1) {
		h := strings.ToLower(strings.TrimRight(m[1], ".-"))
		if h == "" || seen[h] {
			continue
		}
		seen[h] = true
		out = append(out, h)
		if len(out) == maxMentions {
			break
		}
	}
	return out
}

func collectIDs(ctx context.Context, q db, sql string, args ...any) ([]uuid.UUID, error) {
	rows, err := q.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
}

func mentionedUsers(ctx context.Context, q db, body string) ([]uuid.UUID, error) {
	handles := mentionedHandles(body)
	if len(handles) == 0 {
		return nil, nil
	}
	return collectIDs(ctx, q, `SELECT id FROM users WHERE lower(handle) = ANY($1)`, handles)
}

// recipients collects one notification per user, keeping the first (most
// specific) reason a user is notified and never notifying the actor.
type recipients struct {
	actor uuid.UUID
	seen  map[uuid.UUID]bool
	out   []notifications.New
	base  notifications.New
}

func newRecipients(actor uuid.UUID, anonymous bool, postID uuid.UUID, commentID *uuid.UUID) *recipients {
	return &recipients{
		actor: actor,
		seen:  map[uuid.UUID]bool{actor: true},
		base: notifications.New{ActorID: &actor, ActorAnonymous: anonymous,
			PostID: postID, CommentID: commentID},
	}
}

func (r *recipients) add(kind string, users ...uuid.UUID) {
	for _, u := range users {
		if r.seen[u] {
			continue
		}
		r.seen[u] = true
		n := r.base
		n.UserID, n.Kind = u, kind
		r.out = append(r.out, n)
	}
}

// commentEvent describes a new comment for fan-out.
type commentEvent struct {
	postID, postAuthor uuid.UUID
	commentID          uuid.UUID
	actor              uuid.UUID
	parentAuthor       *uuid.UUID // set for replies
	body               string
	anonymous          bool
}

// notifyNewComment tells, in order of precedence: the author of the comment
// being replied to, the post's author, anyone @mentioned, and the post's
// followers. Each person hears once, and the commenter never hears about
// their own comment.
func notifyNewComment(ctx context.Context, q db, e commentEvent) error {
	r := newRecipients(e.actor, e.anonymous, e.postID, &e.commentID)
	if e.parentAuthor != nil {
		r.add(notifications.CommentReply, *e.parentAuthor)
	}
	r.add(notifications.PostReply, e.postAuthor)
	mentioned, err := mentionedUsers(ctx, q, e.body)
	if err != nil {
		return err
	}
	r.add(notifications.Mention, mentioned...)
	followers, err := collectIDs(ctx, q, `SELECT user_id FROM forum_post_follows WHERE post_id = $1`, e.postID)
	if err != nil {
		return err
	}
	r.add(notifications.PostComment, followers...)
	return notifications.Insert(ctx, q, r.out)
}

// notifyNewPost tells anyone @mentioned in a new post.
func notifyNewPost(ctx context.Context, q db, postID, author uuid.UUID, body string, anonymous bool) error {
	mentioned, err := mentionedUsers(ctx, q, body)
	if err != nil {
		return err
	}
	r := newRecipients(author, anonymous, postID, nil)
	r.add(notifications.Mention, mentioned...)
	return notifications.Insert(ctx, q, r.out)
}

// reachedMilestone returns the highest milestone at or below score, or 0.
func reachedMilestone(score int) int {
	best := 0
	for _, m := range forumMilestones {
		if score >= m {
			best = m
		}
	}
	return best
}
