package votes

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"

	"codritium/backend/internal/platform/testutil"
)

func TestDeltas(t *testing.T) {
	cases := []struct{ prev, next, up, down int }{
		{0, 1, 1, 0}, {0, -1, 0, 1}, {1, 0, -1, 0}, {-1, 0, 0, -1},
		{1, -1, -1, 1}, {-1, 1, 1, -1}, {1, 1, 0, 0}, {0, 0, 0, 0},
	}
	for _, c := range cases {
		if up, down := Deltas(c.prev, c.next); up != c.up || down != c.down {
			t.Errorf("%d->%d: got %d/%d want %d/%d", c.prev, c.next, up, down, c.up, c.down)
		}
	}
}

// Concurrent votes by many users must land exactly once each, and one
// user toggling must never double-count (problem comments used to read
// the previous vote without a row lock).
func TestApply_CountersStayConsistent(t *testing.T) {
	pool := testutil.Pool(t)
	ctx := context.Background()
	author := testutil.NewUser(t, pool, "user")
	var slug string
	if err := pool.QueryRow(ctx, `SELECT slug FROM problems ORDER BY slug LIMIT 1`).Scan(&slug); err != nil {
		t.Skip("needs a seeded problem")
	}
	var commentID uuid.UUID
	if err := pool.QueryRow(ctx,
		`INSERT INTO comments (problem_slug, user_id, body) VALUES ($1, $2, 'x') RETURNING id`,
		slug, author.ID).Scan(&commentID); err != nil {
		t.Fatal(err)
	}

	voters := make([]uuid.UUID, 8)
	for i := range voters {
		voters[i] = testutil.NewUser(t, pool, "user").ID
	}
	var wg sync.WaitGroup
	for _, id := range voters {
		wg.Add(1)
		go func(id uuid.UUID) {
			defer wg.Done()
			if _, err := Apply(ctx, pool, ProblemComment, commentID, id, 1); err != nil {
				t.Error(err)
			}
		}(id)
	}
	wg.Wait()

	// One voter flips to down twice (second is a no-op), then clears.
	for _, v := range []int{-1, -1, 0} {
		if _, err := Apply(ctx, pool, ProblemComment, commentID, voters[0], v); err != nil {
			t.Fatal(err)
		}
	}
	var up, down int
	if err := pool.QueryRow(ctx, `SELECT upvotes, downvotes FROM comments WHERE id = $1`, commentID).Scan(&up, &down); err != nil {
		t.Fatal(err)
	}
	if up != 7 || down != 0 {
		t.Fatalf("counters up=%d down=%d, want 7/0", up, down)
	}

	if _, err := Apply(ctx, pool, ProblemComment, uuid.New(), voters[1], 1); err != ErrNotFound {
		t.Fatalf("missing target: err=%v", err)
	}
}
