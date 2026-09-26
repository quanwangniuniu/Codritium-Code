package llm

import (
	"regexp"
	"strings"
)

// ChatJailbreakClassifier is a deliberately narrow filter for the chat_v2
// agent. Unlike the tips-agent tutor which has to hold a Socratic posture
// against rich social-engineering attempts, chat_v2 is a coding partner —
// most prompts are legitimate. The classifier only intercepts attempts to
// reach the hidden test bundle or otherwise short-circuit the grader.
//
// The implementation is regex-based on purpose: a per-turn LLM call would
// cost more than the underlying chat turn, and the severe pattern surface
// is narrow enough that a small handwritten set already covers it.
type ChatJailbreakClassifier struct {
	patterns []*regexp.Regexp
}

// NewChatJailbreakClassifier returns a classifier loaded with the default
// severe pattern list.
func NewChatJailbreakClassifier() *ChatJailbreakClassifier {
	sources := []string{
		// Read or reveal the hidden test bundle.
		`(?i)\b(read|show|display|reveal|leak|see|view|print|cat|open|dump)\b.{0,50}\b(hidden\s*test|test[_\s]answer|hidden_test_content|hidden_test_file|secret\s*test|oracle)\b`,
		`(?i)\b(test_answer\.py|hidden_test_file|hidden_test_content)\b`,
		`(?i)\bgive\s+me\s+the\s+(hidden\s*)?test\s*cases?\b`,
		// Bypass the grader or sandbox.
		`(?i)\bbypass\b.{0,40}\b(grader|evaluator|hidden|sandbox|judge)\b`,
		`(?i)\b(disable|skip|turn\s*off|short[-\s]?circuit)\b.{0,40}\b(grader|evaluator|sandbox|judge)\b`,
		// Forge a result.
		`(?i)\b(forge|fake|spoof|fabricate)\b.{0,30}\b(grader|score|result|verdict)\b`,
		`(?i)\b(write|set|return|output|stub|monkeypatch)\b.{0,50}\b(final_score|scores?|grade)\b.{0,30}(100|perfect|max|true)`,
	}
	c := &ChatJailbreakClassifier{}
	for _, s := range sources {
		re, err := regexp.Compile(s)
		if err != nil {
			continue
		}
		c.patterns = append(c.patterns, re)
	}
	return c
}

// IsSevere returns true when the message matches one of the severe patterns.
// A nil receiver never matches, so callers that never wired a classifier
// stay on the permissive path.
func (c *ChatJailbreakClassifier) IsSevere(message string) bool {
	if c == nil || len(c.patterns) == 0 {
		return false
	}
	trimmed := strings.TrimSpace(message)
	if trimmed == "" {
		return false
	}
	for _, re := range c.patterns {
		if re.MatchString(trimmed) {
			return true
		}
	}
	return false
}
