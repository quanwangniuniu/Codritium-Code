package grader

import (
	"regexp"
	"strings"
)

// Sanitize provenance metadata from grader input to reduce
// shortcut-bias risk. Strips author attribution, version labels,
// timestamps, and model self-references from code comments and
// prompt history before feeding to an LLM judge.

var (
	// Comment lines attributing authorship: requires a proper-name token after the verb
	// so plain phrases like "// sorted by name" are left intact.
	reAuthor = regexp.MustCompile(`(?im)^\s*(//|#|/\*|\*)?\s*(@?author\s*:?\s*[A-Z]|written\s+by\s+[A-Z]|created\s+by\s+[A-Z])[^\n]{0,80}$`)

	// Comment lines with explicit date markers: "Created: 2024-...", "Last modified: ...".
	reDate = regexp.MustCompile(`(?im)^\s*(//|#|/\*|\*)?\s*(created|date|last\s+modified|updated|generated)\s*:?\s*\d{4}[-/]\d{1,2}[-/]\d{1,2}[^\n]*$`)

	// Comment lines with version markers: "v2 of", "rewrite of", "version 3.0".
	reVersion = regexp.MustCompile(`(?im)^\s*(//|#|/\*|\*)?\s*(v\d+\s+of|version\s+\d+(\.\d+)?(\s+of)?|rewrite\s+of|refactored\s+from)\s+[^\n]{1,80}$`)

	// AI brand self-references inside chat history. Conservative list: bare-token "cursor"
	// is excluded because it collides with the common programming concept; project codename
	// words like "sonnet"/"haiku"/"opus" are kept only when they appear as Claude product
	// names with a leading "claude" qualifier (handled by the claude pattern).
	reModelMention = regexp.MustCompile(`(?i)\b(chatgpt|claude(?:\s+(?:sonnet|haiku|opus|\d(?:\.\d)?))?|gpt[-\s]?\d(\.\d)?|gemini(?:\s+\d(?:\.\d)?)?|copilot)\b`)
)

// SanitizeCode removes provenance/version comments line-by-line.
// Preserves all other content untouched.
func SanitizeCode(src string) string {
	lines := strings.Split(src, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if reAuthor.MatchString(line) || reDate.MatchString(line) || reVersion.MatchString(line) {
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

// SanitizeCodeFiles applies SanitizeCode to every file's content.
func SanitizeCodeFiles(files map[string]string) map[string]string {
	if files == nil {
		return nil
	}
	cleaned := make(map[string]string, len(files))
	for name, body := range files {
		cleaned[name] = SanitizeCode(body)
	}
	return cleaned
}

// SanitizeHistory rewrites model brand names to a neutral "AI assistant"
// to neutralize provenance hierarchy bias (Expert > Human > LLM observed
// in Marioriyad 2025). Other content is unchanged.
func SanitizeHistory(items []PromptHistoryItem) []PromptHistoryItem {
	out := make([]PromptHistoryItem, len(items))
	for i, it := range items {
		out[i] = PromptHistoryItem{
			Role:    it.Role,
			Content: reModelMention.ReplaceAllString(it.Content, "AI assistant"),
		}
	}
	return out
}

// SanitizeInput applies all sanitizers to a GraderInput, returning a new
// copy. The original is not mutated.
func SanitizeInput(in GraderInput) GraderInput {
	out := in
	out.StarterFiles = SanitizeCodeFiles(in.StarterFiles)
	out.CandidateFiles = SanitizeCodeFiles(in.CandidateFiles)
	out.PromptHistory = SanitizeHistory(in.PromptHistory)
	return out
}
