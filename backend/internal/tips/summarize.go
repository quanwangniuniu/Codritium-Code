package tips

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SummarizeSessionEvents reads the agent chat event stream for a session and
// returns a short markdown digest the tips-agent can fold into its system
// prompt. The summary is rule-based — no LLM call — and is intentionally
// coarse (counts, not transcripts) so it stays cheap and safe to embed in
// a tutor turn.
//
// Returns an empty string when the session has no chat activity yet, so a
// caller can drop the summary section unconditionally.
func SummarizeSessionEvents(ctx context.Context, pool *pgxpool.Pool, sessionID uuid.UUID) (string, error) {
	rows, err := pool.Query(ctx, `
		SELECT kind, payload::text
		FROM session_events
		WHERE session_id = $1
		ORDER BY seq ASC`, sessionID)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	var (
		toolUses           int
		reads              int
		edits              int
		runTests           int
		grepGlobCmd        int
		approves           int
		modifies           int
		rejects            int
		pushbacks          int
		reverts            int
		testRuns           int
		candidateRanTests  int
		agentRanTests      int
		lastTestPassed     int
		lastTestFailed     int
		hasTestRun         bool
		planEntered        int
		planExited         int
		selfCheckArtifacts int
		turnCount          int
	)

	for rows.Next() {
		var kind, payloadStr string
		if err := rows.Scan(&kind, &payloadStr); err != nil {
			continue
		}
		var payload map[string]any
		if err := json.Unmarshal([]byte(payloadStr), &payload); err != nil {
			continue
		}
		switch kind {
		case "tool_use_proposed":
			toolUses++
			tool, _ := payload["tool"].(string)
			switch tool {
			case "FileRead":
				reads++
			case "FileEdit":
				edits++
			case "RunTests":
				runTests++
			case "Grep", "Glob", "RunCommand":
				grepGlobCmd++
			}
		case "candidate_approved":
			auto, _ := payload["auto"].(bool)
			if auto {
				continue
			}
			approves++
			if mod, _ := payload["modified"].(bool); mod {
				modifies++
			}
		case "candidate_rejected":
			rejects++
		case "candidate_pushed_back":
			pushbacks++
		case "candidate_reverted_edit":
			reverts++
		case "test_executed":
			testRuns++
			hasTestRun = true
			lastTestPassed = intField(payload, "passed")
			lastTestFailed = intField(payload, "failed")
			trigger, _ := payload["trigger"].(string)
			switch trigger {
			case "candidate":
				candidateRanTests++
			case "agent":
				agentRanTests++
			}
		case "plan_mode_entered":
			planEntered++
		case "plan_mode_exited":
			planExited++
		case "self_check_artifact":
			selfCheckArtifacts++
		case "turn_completed":
			turnCount++
		}
	}

	if toolUses+approves+rejects+testRuns+turnCount == 0 {
		return "", nil
	}

	var lines []string

	if turnCount > 0 {
		lines = append(lines, fmt.Sprintf("- Chat turns completed so far: %d.", turnCount))
	}

	var activity []string
	if reads > 0 {
		activity = append(activity, fmt.Sprintf("read %d file(s)", reads))
	}
	if edits > 0 {
		activity = append(activity, fmt.Sprintf("proposed %d edit(s)", edits))
	}
	if runTests > 0 {
		activity = append(activity, fmt.Sprintf("proposed %d test run(s)", runTests))
	}
	if grepGlobCmd > 0 {
		activity = append(activity, fmt.Sprintf("used grep/glob/command %d time(s)", grepGlobCmd))
	}
	if len(activity) > 0 {
		lines = append(lines, "- AI tool activity: "+strings.Join(activity, ", ")+".")
	}

	if approves+modifies+rejects+pushbacks+reverts > 0 {
		var decisions []string
		if approves > 0 {
			decisions = append(decisions, fmt.Sprintf("approved %d", approves))
		}
		if modifies > 0 {
			decisions = append(decisions, fmt.Sprintf("modified %d", modifies))
		}
		if rejects > 0 {
			decisions = append(decisions, fmt.Sprintf("rejected %d", rejects))
		}
		if pushbacks > 0 {
			decisions = append(decisions, fmt.Sprintf("pushed back %d", pushbacks))
		}
		if reverts > 0 {
			decisions = append(decisions, fmt.Sprintf("reverted %d", reverts))
		}
		lines = append(lines, "- Candidate decisions: "+strings.Join(decisions, ", ")+".")
	}

	if testRuns > 0 {
		ran := fmt.Sprintf("- Test runs: %d total", testRuns)
		if candidateRanTests > 0 || agentRanTests > 0 {
			ran += fmt.Sprintf(" (candidate %d, agent %d)", candidateRanTests, agentRanTests)
		}
		ran += "."
		lines = append(lines, ran)
		if hasTestRun {
			lines = append(lines, fmt.Sprintf("- Last test run: %d passed, %d failed.", lastTestPassed, lastTestFailed))
		}
	}

	if planEntered > 0 {
		plan := fmt.Sprintf("- Plan mode entered %d time(s)", planEntered)
		if planExited < planEntered {
			plan += " — currently still inside plan mode"
		}
		plan += "."
		lines = append(lines, plan)
	}

	if selfCheckArtifacts > 0 {
		lines = append(lines, fmt.Sprintf("- Self-check artifacts produced: %d.", selfCheckArtifacts))
	}

	return strings.Join(lines, "\n"), nil
}

func intField(payload map[string]any, key string) int {
	v, ok := payload[key]
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	}
	return 0
}
