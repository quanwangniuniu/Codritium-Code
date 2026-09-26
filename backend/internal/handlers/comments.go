package handlers

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"codritium/backend/internal/auth"
)

type CommentsDeps struct {
	Pool *pgxpool.Pool
}

type commentItem struct {
	ID             uuid.UUID  `json:"id"`
	ProblemSlug    string     `json:"problem_slug"`
	UserID         uuid.UUID  `json:"user_id"`
	UserHandle     string     `json:"user_handle"`
	UserDisplay    string     `json:"user_display_name"`
	UserAvatarURL  string     `json:"user_avatar_url"`
	UserTier       string     `json:"user_tier"`
	Body           string     `json:"body"`
	Upvotes        int        `json:"upvotes"`
	Downvotes      int        `json:"downvotes"`
	ParentID       *uuid.UUID `json:"parent_id,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	MyVote         int        `json:"my_vote"`
}

// ListComments returns a page of comments for a problem, newest first. The
// gate matches the reply gate: callers must have completed (graded) the
// problem unless they are admin. cursor is opaque base64; clients send back
// the next_cursor returned by the previous page.
func ListComments(deps CommentsDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slug := r.PathValue("slug")
		if slug == "" {
			http.Error(w, "slug required", http.StatusBadRequest)
			return
		}
		u := auth.FromContext(r.Context())
		if u == nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if err := requireGradedOrAdmin(r, deps.Pool, u, slug); err != nil {
			writeJSON(w, http.StatusForbidden, map[string]any{
				"error":          "must_complete_problem",
				"challenge_slug": slug,
			})
			return
		}

		limit := 20
		if v := r.URL.Query().Get("limit"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 50 {
				limit = n
			}
		}

		cursorTS, cursorID, cursorOK := decodeCommentsCursor(r.URL.Query().Get("cursor"))

		var (
			rows pgx.Rows
			err  error
		)
		if cursorOK {
			rows, err = deps.Pool.Query(r.Context(), `
				SELECT c.id, c.problem_slug, c.user_id,
				       u.handle, u.display_name, COALESCE(u.avatar_url,''), u.tier,
				       c.body, c.upvotes, c.downvotes, c.parent_id, c.created_at,
				       COALESCE(v.value, 0)
				FROM comments c
				JOIN users u ON u.id = c.user_id
				LEFT JOIN comment_votes v ON v.comment_id = c.id AND v.user_id = $1
				WHERE c.problem_slug = $2
				  AND c.deleted_at IS NULL
				  AND (c.created_at, c.id) < ($3, $4)
				ORDER BY c.created_at DESC, c.id DESC
				LIMIT $5`,
				u.ID, slug, cursorTS, cursorID, limit+1)
		} else {
			rows, err = deps.Pool.Query(r.Context(), `
				SELECT c.id, c.problem_slug, c.user_id,
				       u.handle, u.display_name, COALESCE(u.avatar_url,''), u.tier,
				       c.body, c.upvotes, c.downvotes, c.parent_id, c.created_at,
				       COALESCE(v.value, 0)
				FROM comments c
				JOIN users u ON u.id = c.user_id
				LEFT JOIN comment_votes v ON v.comment_id = c.id AND v.user_id = $1
				WHERE c.problem_slug = $2
				  AND c.deleted_at IS NULL
				ORDER BY c.created_at DESC, c.id DESC
				LIMIT $3`,
				u.ID, slug, limit+1)
		}
		if err != nil {
			http.Error(w, "list comments: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		items := make([]commentItem, 0, limit)
		for rows.Next() {
			var it commentItem
			if err := rows.Scan(&it.ID, &it.ProblemSlug, &it.UserID,
				&it.UserHandle, &it.UserDisplay, &it.UserAvatarURL, &it.UserTier,
				&it.Body, &it.Upvotes, &it.Downvotes, &it.ParentID, &it.CreatedAt,
				&it.MyVote); err != nil {
				http.Error(w, "scan comment: "+err.Error(), http.StatusInternalServerError)
				return
			}
			items = append(items, it)
		}
		next := ""
		if len(items) > limit {
			tail := items[limit-1]
			next = encodeCommentsCursor(tail.CreatedAt, tail.ID)
			items = items[:limit]
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"comments":    items,
			"next_cursor": next,
		})
	}
}

type createCommentRequest struct {
	Body     string     `json:"body"`
	ParentID *uuid.UUID `json:"parent_id,omitempty"`
}

// CreateComment inserts a new comment for the caller. Honors the same graded
// gate as ListComments and rejects empty bodies. parent_id is validated to
// belong to the same problem so a reply can't escape its thread.
func CreateComment(deps CommentsDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slug := r.PathValue("slug")
		if slug == "" {
			http.Error(w, "slug required", http.StatusBadRequest)
			return
		}
		u := auth.FromContext(r.Context())
		if u == nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if err := requireGradedOrAdmin(r, deps.Pool, u, slug); err != nil {
			writeJSON(w, http.StatusForbidden, map[string]any{
				"error":          "must_complete_problem",
				"challenge_slug": slug,
			})
			return
		}
		var req createCommentRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad json: "+err.Error(), http.StatusBadRequest)
			return
		}
		body := strings.TrimSpace(req.Body)
		if body == "" {
			http.Error(w, "body required", http.StatusBadRequest)
			return
		}

		if req.ParentID != nil {
			var parentSlug string
			err := deps.Pool.QueryRow(r.Context(),
				`SELECT problem_slug FROM comments WHERE id = $1 AND deleted_at IS NULL`,
				*req.ParentID,
			).Scan(&parentSlug)
			if err != nil || parentSlug != slug {
				http.Error(w, "parent comment not in this problem", http.StatusBadRequest)
				return
			}
		}

		var id uuid.UUID
		var createdAt time.Time
		err := deps.Pool.QueryRow(r.Context(), `
			INSERT INTO comments (problem_slug, user_id, body, parent_id)
			VALUES ($1, $2, $3, $4)
			RETURNING id, created_at`,
			slug, u.ID, body, req.ParentID,
		).Scan(&id, &createdAt)
		if err != nil {
			http.Error(w, "create comment: "+err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{
			"id":         id,
			"created_at": createdAt,
		})
	}
}

type voteRequest struct {
	Value int `json:"value"`
}

// VoteComment upserts the caller's vote and atomically updates the cached
// upvote/downvote counters on the comment row. value=0 removes any
// existing vote. Returns the fresh counter pair so the client can update
// its UI without re-fetching.
func VoteComment(deps CommentsDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := r.PathValue("id")
		commentID, err := uuid.Parse(idStr)
		if err != nil {
			http.Error(w, "bad id", http.StatusBadRequest)
			return
		}
		u := auth.FromContext(r.Context())
		if u == nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		var req voteRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad json: "+err.Error(), http.StatusBadRequest)
			return
		}
		if req.Value != -1 && req.Value != 0 && req.Value != 1 {
			http.Error(w, "value must be -1, 0, or 1", http.StatusBadRequest)
			return
		}

		tx, err := deps.Pool.BeginTx(r.Context(), pgx.TxOptions{})
		if err != nil {
			http.Error(w, "tx begin: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer tx.Rollback(r.Context())

		var prev int
		err = tx.QueryRow(r.Context(),
			`SELECT value FROM comment_votes WHERE comment_id = $1 AND user_id = $2`,
			commentID, u.ID,
		).Scan(&prev)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "read vote: "+err.Error(), http.StatusInternalServerError)
			return
		}

		switch {
		case req.Value == 0 && prev != 0:
			if _, err := tx.Exec(r.Context(),
				`DELETE FROM comment_votes WHERE comment_id = $1 AND user_id = $2`,
				commentID, u.ID,
			); err != nil {
				http.Error(w, "delete vote: "+err.Error(), http.StatusInternalServerError)
				return
			}
		case req.Value != 0 && prev == 0:
			if _, err := tx.Exec(r.Context(),
				`INSERT INTO comment_votes (comment_id, user_id, value) VALUES ($1, $2, $3)`,
				commentID, u.ID, req.Value,
			); err != nil {
				http.Error(w, "insert vote: "+err.Error(), http.StatusInternalServerError)
				return
			}
		case req.Value != 0 && prev != req.Value:
			if _, err := tx.Exec(r.Context(),
				`UPDATE comment_votes SET value = $3, voted_at = now() WHERE comment_id = $1 AND user_id = $2`,
				commentID, u.ID, req.Value,
			); err != nil {
				http.Error(w, "update vote: "+err.Error(), http.StatusInternalServerError)
				return
			}
		}

		upDelta, downDelta := voteDeltas(prev, req.Value)
		var up, down int
		err = tx.QueryRow(r.Context(),
			`UPDATE comments SET upvotes = upvotes + $2, downvotes = downvotes + $3
			 WHERE id = $1
			 RETURNING upvotes, downvotes`,
			commentID, upDelta, downDelta,
		).Scan(&up, &down)
		if err != nil {
			http.Error(w, "update counters: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if err := tx.Commit(r.Context()); err != nil {
			http.Error(w, "tx commit: "+err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"upvotes":   up,
			"downvotes": down,
			"my_vote":   req.Value,
		})
	}
}

// DeleteComment soft-deletes the caller's own comment. Returns 404 either
// when the row doesn't exist or belongs to someone else; the response
// shouldn't reveal which.
func DeleteComment(deps CommentsDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := r.PathValue("id")
		id, err := uuid.Parse(idStr)
		if err != nil {
			http.Error(w, "bad id", http.StatusBadRequest)
			return
		}
		u := auth.FromContext(r.Context())
		if u == nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		tag, err := deps.Pool.Exec(r.Context(),
			`UPDATE comments SET deleted_at = now() WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL`,
			id, u.ID)
		if err != nil {
			http.Error(w, "delete: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if tag.RowsAffected() == 0 {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// requireGradedOrAdmin returns nil if the caller is admin or has at least one
// graded session on this problem; otherwise returns an error the caller maps
// to a 403.
func requireGradedOrAdmin(r *http.Request, pool *pgxpool.Pool, u *auth.User, slug string) error {
	if u.Role == "admin" {
		return nil
	}
	var ok bool
	if err := pool.QueryRow(r.Context(), `
		SELECT EXISTS(
		  SELECT 1 FROM candidate_sessions
		  WHERE candidate_id = $1 AND challenge_id = $2 AND graded_at IS NOT NULL
		)`, u.Handle, slug,
	).Scan(&ok); err != nil {
		return err
	}
	if !ok {
		return errors.New("not graded")
	}
	return nil
}

func voteDeltas(prev, next int) (upDelta, downDelta int) {
	if prev == 1 {
		upDelta--
	} else if prev == -1 {
		downDelta--
	}
	if next == 1 {
		upDelta++
	} else if next == -1 {
		downDelta++
	}
	return
}

const cursorSep = "|"

func encodeCommentsCursor(ts time.Time, id uuid.UUID) string {
	raw := ts.UTC().Format(time.RFC3339Nano) + cursorSep + id.String()
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodeCommentsCursor(cursor string) (time.Time, uuid.UUID, bool) {
	if cursor == "" {
		return time.Time{}, uuid.Nil, false
	}
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return time.Time{}, uuid.Nil, false
	}
	parts := strings.SplitN(string(raw), cursorSep, 2)
	if len(parts) != 2 {
		return time.Time{}, uuid.Nil, false
	}
	ts, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return time.Time{}, uuid.Nil, false
	}
	id, err := uuid.Parse(parts[1])
	if err != nil {
		return time.Time{}, uuid.Nil, false
	}
	return ts, id, true
}
