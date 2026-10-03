package grading

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"codritium/backend/internal/grader"
)

// loadPromptHistory returns the chat transcript of the submission's
// session, flattened for the grader. Submissions without a session (or
// from before chat_v2) have no history.
func loadPromptHistory(ctx context.Context, pool *pgxpool.Pool, submissionID uuid.UUID) ([]grader.PromptHistoryItem, error) {
	var sessionID *uuid.UUID
	if err := pool.QueryRow(ctx,
		`SELECT session_id FROM submissions WHERE id = $1`, submissionID,
	).Scan(&sessionID); err != nil || sessionID == nil {
		return nil, err
	}
	rows, err := pool.Query(ctx, `
		SELECT role, content::text FROM session_messages
		WHERE session_id = $1 ORDER BY seq ASC`, *sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var hist []grader.PromptHistoryItem
	for rows.Next() {
		var role string
		var content []byte
		if err := rows.Scan(&role, &content); err != nil {
			return nil, err
		}
		if text := flattenBlocks(content); text != "" {
			hist = append(hist, grader.PromptHistoryItem{Role: role, Content: text})
		}
	}
	return hist, rows.Err()
}

// flattenBlocks turns a session_messages.content array (text / tool_use /
// tool_result blocks, as written by llm.Agent) into plain text. tool_use
// becomes "<tool:Name>" so the grader sees intent without payloads;
// tool_result is dropped — it would dominate the prompt budget and is
// implicit in the next turn. A bare JSON string is returned as-is.
func flattenBlocks(raw []byte) string {
	var blocks []map[string]any
	if err := json.Unmarshal(raw, &blocks); err != nil {
		var s string
		if json.Unmarshal(raw, &s) == nil {
			return s
		}
		return ""
	}
	var parts []string
	for _, b := range blocks {
		switch b["type"] {
		case "text":
			if s, _ := b["text"].(string); s != "" {
				parts = append(parts, s)
			}
		case "tool_use":
			if name, _ := b["name"].(string); name != "" {
				parts = append(parts, "<tool:"+name+">")
			}
		}
	}
	return strings.Join(parts, "\n")
}
