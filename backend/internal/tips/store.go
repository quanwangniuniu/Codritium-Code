package tips

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ProblemContext is the candidate-safe view of a problem the tips-agent is
// allowed to read. The shape intentionally omits hidden_test_filename and
// hidden_test_content so the agent literally has no access path to them —
// L4 data-flow isolation per research/R14_ai_defense §8.1.
type ProblemContext struct {
	Slug        string
	Title       string
	Difficulty  string
	ReadmeMD    string
	SoulPrebake string
}

// SessionContext bundles the data the tips handler needs to satisfy one
// turn: the session lifecycle stage plus the candidate-safe problem view.
type SessionContext struct {
	SessionID uuid.UUID
	UserID    uuid.UUID
	Mode      Mode
	Problem   ProblemContext
	Submitted bool
}

// ErrSessionNotFound is returned when LoadSessionContext cannot resolve
// the session_id. Callers should translate to HTTP 404.
var ErrSessionNotFound = errors.New("tips: session not found")

// LoadSessionContext fetches the candidate-safe context for a tips turn.
// The SELECT list is the contract — adding hidden_test_file here would
// require a code review that surfaces the leak intentionally.
func LoadSessionContext(ctx context.Context, pool *pgxpool.Pool, sessionID uuid.UUID) (SessionContext, error) {
	if pool == nil {
		return SessionContext{}, errors.New("tips: pool not configured")
	}

	var (
		sc          SessionContext
		mode        string
		submittedAt *string
	)
	sc.SessionID = sessionID
	err := pool.QueryRow(ctx, `
		SELECT
			cs.user_id,
			cs.mode,
			cs.submitted_at::text,
			p.slug,
			p.title,
			p.difficulty,
			p.readme_md,
			p.soul_prebake
		FROM candidate_sessions cs
		JOIN problems p ON p.slug = cs.challenge_id
		WHERE cs.session_id = $1`,
		sessionID,
	).Scan(
		&sc.UserID,
		&mode,
		&submittedAt,
		&sc.Problem.Slug,
		&sc.Problem.Title,
		&sc.Problem.Difficulty,
		&sc.Problem.ReadmeMD,
		&sc.Problem.SoulPrebake,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return SessionContext{}, ErrSessionNotFound
	}
	if err != nil {
		return SessionContext{}, fmt.Errorf("tips: load session %s: %w", sessionID, err)
	}
	sc.Mode = Mode(mode)
	sc.Submitted = submittedAt != nil
	return sc, nil
}

// StoredTurn is one persisted tutor-conversation row. seq is monotonic
// per session and assigned at write time; created_at is RFC3339.
type StoredTurn struct {
	Seq       int
	Role      string
	Text      string
	CreatedAt string
}

// LoadTipsHistory returns every tips turn for the session in seq order.
// An empty slice (not nil) is returned when the session has no tips
// rows yet — callers can iterate without a nil guard.
func LoadTipsHistory(ctx context.Context, pool *pgxpool.Pool, sessionID uuid.UUID) ([]StoredTurn, error) {
	if pool == nil {
		return nil, errors.New("tips: pool not configured")
	}
	rows, err := pool.Query(ctx, `
		SELECT seq, role, text, created_at::text
		FROM tips_messages
		WHERE session_id = $1
		ORDER BY seq ASC`,
		sessionID,
	)
	if err != nil {
		return nil, fmt.Errorf("tips: load history %s: %w", sessionID, err)
	}
	defer rows.Close()

	out := make([]StoredTurn, 0)
	for rows.Next() {
		var t StoredTurn
		if err := rows.Scan(&t.Seq, &t.Role, &t.Text, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("tips: scan history row: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("tips: iterate history: %w", err)
	}
	return out, nil
}

// AppendTipsTurn writes one turn at the next available seq for the
// session. The seq assignment is done in SQL with a single UPDATE-style
// expression so two concurrent writes for the same session cannot
// collide on the (session_id, seq) primary key.
func AppendTipsTurn(ctx context.Context, pool *pgxpool.Pool, sessionID uuid.UUID, role, text string) (StoredTurn, error) {
	if pool == nil {
		return StoredTurn{}, errors.New("tips: pool not configured")
	}
	if role != "user" && role != "model" {
		return StoredTurn{}, fmt.Errorf("tips: invalid role %q", role)
	}
	var t StoredTurn
	t.Role = role
	t.Text = text
	err := pool.QueryRow(ctx, `
		INSERT INTO tips_messages (session_id, seq, role, text)
		SELECT $1,
		       COALESCE(MAX(seq), 0) + 1,
		       $2,
		       $3
		FROM tips_messages WHERE session_id = $1
		RETURNING seq, created_at::text`,
		sessionID, role, text,
	).Scan(&t.Seq, &t.CreatedAt)
	if err != nil {
		return StoredTurn{}, fmt.Errorf("tips: append turn: %w", err)
	}
	return t, nil
}
