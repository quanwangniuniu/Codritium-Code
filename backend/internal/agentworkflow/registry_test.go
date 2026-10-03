package agentworkflow

import (
	"strings"
	"testing"
)

type testValidationContext map[string]string

func (c testValidationContext) Get(path string) (string, bool) {
	content, exists := c[path]
	return content, exists
}

func TestDefaultRegistryApprovalRules(t *testing.T) {
	registry := NewDefaultRegistry()

	tests := []struct {
		name             string
		requiresApproval bool
	}{
		{name: "FileRead", requiresApproval: false},
		{name: "Grep", requiresApproval: false},
		{name: "Glob", requiresApproval: false},
		{name: "FileEdit", requiresApproval: true},
		{name: "RunTests", requiresApproval: true},
		{name: "RunCommand", requiresApproval: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			action, ok := registry.Resolve(Request{Name: test.name})
			if !ok {
				t.Fatalf("expected %s action to exist", test.name)
			}
			if action.RequiresApproval != test.requiresApproval {
				t.Fatalf(
					"expected RequiresApproval=%v, got %v",
					test.requiresApproval,
					action.RequiresApproval,
				)
			}
		})
	}
}

func TestVerifyFileEditInput(t *testing.T) {
	context := testValidationContext{
		"example.py": "value = 1\n",
	}

	input, err := VerifyFileEditInput(Input{
		"path":     "example.py",
		"old_text": "value = 1",
		"new_text": `value = 2\nprint(value)\n`,
	}, context)
	if err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}

	newText, ok := input["new_text"].(string)
	if !ok {
		t.Fatal("expected normalized new_text")
	}
	if !strings.Contains(newText, "\n") {
		t.Fatalf("expected escaped newline to be normalized, got %q", newText)
	}
}

func TestVerifyFileEditInputRejectsMissingOldText(t *testing.T) {
	context := testValidationContext{
		"example.py": "value = 1\n",
	}

	_, err := VerifyFileEditInput(Input{
		"path":     "example.py",
		"new_text": "value = 2",
	}, context)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if err.Error() != "old_text is required when editing an existing file" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNormalizeFileEditNewText(t *testing.T) {
	t.Run("converts multiline escaped text", func(t *testing.T) {
		input := `first line\nsecond line\nthird line`
		want := "first line\nsecond line\nthird line"

		if got := normalizeFileEditNewText(input); got != want {
			t.Fatalf("normalizeFileEditNewText()=%q want %q", got, want)
		}
	})

	t.Run("converts escaped newline before indentation", func(t *testing.T) {
		input := `before\n        if value:`
		want := "before\n        if value:"

		if got := normalizeFileEditNewText(input); got != want {
			t.Fatalf("normalizeFileEditNewText()=%q want %q", got, want)
		}
	})

	t.Run("preserves escaped newline inside one-line code", func(t *testing.T) {
		input := `print("\n")`

		if got := normalizeFileEditNewText(input); got != input {
			t.Fatalf("normalizeFileEditNewText()=%q want unchanged %q", got, input)
		}
	})
}
