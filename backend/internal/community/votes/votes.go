// Package votes is the one implementation of up/down voting, shared by
// forum posts, forum comments, and problem comments.
package votes

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"codritium/backend/internal/platform/httpx"
)

// ErrNotFound means the target does not exist or is deleted.
var ErrNotFound = errors.New("vote target not found")

// Target names a votable table. Only the constants below exist; table and
// column names are never taken from a request.
type Target struct {
	voteTable, targetCol, targetTable string
}

var (
	ForumPost      = Target{"forum_post_votes", "post_id", "forum_posts"}
	ForumComment   = Target{"forum_comment_votes", "comment_id", "forum_comments"}
	ProblemComment = Target{"comment_votes", "comment_id", "comments"}
)

// Result is the target's counters after the vote.
type Result struct {
	Up    int `json:"upvotes"`
	Down  int `json:"downvotes"`
	Score int `json:"score"`
	Mine  int `json:"my_vote"`
}

// Apply sets the user's vote (-1, 0 to clear, 1) and updates the cached
// counters in one transaction. The previous vote is locked so concurrent
// votes cannot double-count.
func Apply(ctx context.Context, pool *pgxpool.Pool, t Target, targetID, userID uuid.UUID, value int) (Result, error) {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Result{}, err
	}
	defer tx.Rollback(ctx)

	var exists bool
	if err := tx.QueryRow(ctx, fmt.Sprintf(
		`SELECT EXISTS(SELECT 1 FROM %s WHERE id = $1 AND deleted_at IS NULL)`, t.targetTable),
		targetID).Scan(&exists); err != nil {
		return Result{}, err
	}
	if !exists {
		return Result{}, ErrNotFound
	}

	var prev int
	err = tx.QueryRow(ctx, fmt.Sprintf(
		`SELECT value FROM %s WHERE %s = $1 AND user_id = $2 FOR UPDATE`, t.voteTable, t.targetCol),
		targetID, userID).Scan(&prev)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return Result{}, err
	}
	switch {
	case value == 0 && prev != 0:
		_, err = tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s WHERE %s = $1 AND user_id = $2`, t.voteTable, t.targetCol),
			targetID, userID)
	case value != 0 && prev == 0:
		_, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s (%s, user_id, value) VALUES ($1, $2, $3)`, t.voteTable, t.targetCol),
			targetID, userID, value)
	case value != 0 && prev != value:
		_, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s SET value = $3, voted_at = now() WHERE %s = $1 AND user_id = $2`, t.voteTable, t.targetCol),
			targetID, userID, value)
	}
	if err != nil {
		return Result{}, err
	}
	upDelta, downDelta := Deltas(prev, value)
	res := Result{Mine: value}
	if err := tx.QueryRow(ctx, fmt.Sprintf(
		`UPDATE %s SET upvotes = upvotes + $2, downvotes = downvotes + $3 WHERE id = $1 RETURNING upvotes, downvotes`, t.targetTable),
		targetID, upDelta, downDelta).Scan(&res.Up, &res.Down); err != nil {
		return Result{}, err
	}
	res.Score = res.Up - res.Down
	return res, tx.Commit(ctx)
}

// Deltas is how the up/down counters change when a vote goes prev -> next.
func Deltas(prev, next int) (upDelta, downDelta int) {
	switch prev {
	case 1:
		upDelta--
	case -1:
		downDelta--
	}
	switch next {
	case 1:
		upDelta++
	case -1:
		downDelta++
	}
	return upDelta, downDelta
}

// DecodeValue reads {"value": -1|0|1}, writing a 400 on anything else.
func DecodeValue(w http.ResponseWriter, r *http.Request) (int, bool) {
	var req struct {
		Value int `json:"value"`
	}
	if !httpx.Decode(w, r, &req) {
		return 0, false
	}
	if req.Value < -1 || req.Value > 1 {
		httpx.BadRequest(w, "invalid_vote", "Vote must be -1, 0, or 1.")
		return 0, false
	}
	return req.Value, true
}

// Handle is the shared vote endpoint body: auth, decode, apply, respond.
func Handle(w http.ResponseWriter, r *http.Request, pool *pgxpool.Pool, t Target, targetID, userID uuid.UUID) {
	value, ok := DecodeValue(w, r)
	if !ok {
		return
	}
	res, err := Apply(r.Context(), pool, t, targetID, userID, value)
	if errors.Is(err, ErrNotFound) {
		httpx.NotFound(w, "Not found.")
		return
	}
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, res)
}
