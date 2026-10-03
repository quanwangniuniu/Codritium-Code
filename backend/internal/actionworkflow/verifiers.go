package actionworkflow

import (
	"fmt"
	"strings"
)

// VerifyFileEditInput validates and normalizes one FileEdit request.
func VerifyFileEditInput(
	input Input,
	context ValidationContext,
) (Input, error) {
	normalized := cloneInput(input)

	path, pathOK := normalized["path"].(string)
	legacyContent, legacyOK := normalized["content"].(string)
	newText, newTextOK := normalized["new_text"].(string)

	if newTextOK {
		normalizedText := normalizeFileEditNewText(newText)
		if normalizedText != newText {
			newText = normalizedText
			normalized["new_text"] = normalizedText
		}
	}

	if !pathOK || path == "" {
		return nil, fmt.Errorf("path is required")
	}

	// Accept the old complete-file format temporarily so existing stored
	// conversations and tests remain compatible during the migration.
	if !legacyOK && !newTextOK {
		return nil, fmt.Errorf(
			"new_text is required; for an existing file also provide exact old_text",
		)
	}
	if legacyOK && legacyContent == "" {
		return nil, fmt.Errorf("legacy content cannot be empty")
	}

	if context != nil {
		current, exists := context.Get(path)

		if legacyOK {
			if exists && current == legacyContent {
				return nil, fmt.Errorf(
					"proposed content for %s is identical to the current file",
					path,
				)
			}
		} else if exists {
			oldText, oldTextOK := normalized["old_text"].(string)
			if !oldTextOK || oldText == "" {
				return nil, fmt.Errorf(
					"old_text is required when editing an existing file",
				)
			}

			matches := strings.Count(current, oldText)
			if matches == 0 {
				return nil, fmt.Errorf(
					"old_text was not found; read the latest file and try again",
				)
			}
			if matches > 1 {
				return nil, fmt.Errorf(
					"old_text matched more than once; include more surrounding context",
				)
			}
			if oldText == newText {
				return nil, fmt.Errorf(
					"the proposed edit would not change the file",
				)
			}
		} else {
			oldText, _ := normalized["old_text"].(string)
			if oldText != "" {
				return nil, fmt.Errorf(
					"cannot replace old_text in a file that does not exist",
				)
			}
			if newText == "" {
				return nil, fmt.Errorf("new file content cannot be empty")
			}
		}
	}

	return normalized, nil
}

func cloneInput(input Input) Input {
	cloned := make(Input, len(input))
	for key, value := range input {
		cloned[key] = value
	}
	return cloned
}

func normalizeFileEditNewText(text string) string {
	// Real multiline content needs no conversion.
	if strings.Contains(text, "\n") {
		return text
	}

	const escapedNewline = `\n`
	if !strings.Contains(text, escapedNewline) {
		return text
	}

	// Multiple escaped newlines normally mean Qwen double-escaped a
	// multiline file. A single escaped newline followed by indentation
	// normally means a multiline code replacement.
	shouldNormalize := strings.Count(text, escapedNewline) >= 2
	if !shouldNormalize {
		_, after, found := strings.Cut(text, escapedNewline)
		shouldNormalize = found &&
			(strings.HasPrefix(after, " ") || strings.HasPrefix(after, "\t"))
	}

	if !shouldNormalize {
		return text
	}

	return strings.ReplaceAll(text, escapedNewline, "\n")
}
