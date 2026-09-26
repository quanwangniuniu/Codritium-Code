package grader

import (
	"strings"
	"testing"
)

func TestCapHistory_Empty(t *testing.T) {
	out := capHistory(nil, 100)
	if len(out) != 0 {
		t.Errorf("expected empty, got %d items", len(out))
	}
}

func TestCapHistory_AllFits(t *testing.T) {
	items := []PromptHistoryItem{
		{Role: "user", Content: "hi"},
		{Role: "assistant", Content: "hello"},
	}
	out := capHistory(items, 1000)
	if len(out) != 2 {
		t.Errorf("expected pass-through 2 items, got %d", len(out))
	}
}

func TestCapHistory_SingleOversizeTurn(t *testing.T) {
	// Single turn that exceeds budget must be truncated, not returned untrimmed.
	big := strings.Repeat("x", 5000)
	items := []PromptHistoryItem{{Role: "user", Content: big}}
	out := capHistory(items, 1000)
	if len(out) != 1 {
		t.Fatalf("expected 1 item, got %d", len(out))
	}
	if len(out[0].Content) > 1000 {
		t.Errorf("expected first turn truncated to <=1000 chars, got %d", len(out[0].Content))
	}
	if !strings.HasSuffix(out[0].Content, "…[truncated]…") {
		t.Errorf("expected truncation marker, got: %q", out[0].Content[max(0, len(out[0].Content)-30):])
	}
}

func TestCapHistory_OversizeRecentDoesNotDropAllOlder(t *testing.T) {
	// Bug from review: a big newest turn used to break the loop, dropping
	// all older small turns. The newest should be truncated and older ones
	// (that fit budget) should still survive.
	items := []PromptHistoryItem{
		{Role: "user", Content: "first turn small"},
		{Role: "assistant", Content: "small"},
		{Role: "user", Content: "small2"},
		{Role: "assistant", Content: "small3"},
		{Role: "user", Content: strings.Repeat("x", 5000)},
	}
	out := capHistory(items, 1500)

	// First turn must be present.
	if out[0].Content != "first turn small" {
		t.Errorf("first turn missing or modified: %q", out[0].Content)
	}
	// Newest (big) turn must appear, truncated.
	last := out[len(out)-1]
	if last.Role != "user" {
		t.Errorf("expected newest turn role=user, got %q", last.Role)
	}
	if !strings.HasSuffix(last.Content, "…[truncated]…") {
		t.Errorf("expected newest turn truncated: %q", last.Content[max(0, len(last.Content)-30):])
	}
	// Total chars within budget.
	total := 0
	for _, it := range out {
		total += len(it.Content)
	}
	if total > 1500 {
		t.Errorf("total chars %d exceeded budget 1500", total)
	}
}

func TestCapHistory_MultipleTurnsFitTail(t *testing.T) {
	items := []PromptHistoryItem{
		{Role: "user", Content: "first"},
		{Role: "assistant", Content: "mid1"},
		{Role: "user", Content: "mid2"},
		{Role: "assistant", Content: "last response"},
	}
	out := capHistory(items, 500)
	// Everything fits; pass-through expected.
	if len(out) != 4 {
		t.Errorf("expected 4-item pass-through, got %d", len(out))
	}
}

func TestCapFileMap_TruncatesOversize(t *testing.T) {
	files := map[string]string{
		"a.py": strings.Repeat("a", 100),
		"b.py": strings.Repeat("b", 9000),
	}
	out := capFileMap(files, 5000)
	if len(out["a.py"]) != 100 {
		t.Errorf("small file should pass through unchanged, got len=%d", len(out["a.py"]))
	}
	if !strings.HasSuffix(out["b.py"], "…[truncated]…") {
		t.Errorf("large file should be truncated with marker")
	}
}

func TestSchemaForDim_AllDimsCovered(t *testing.T) {
	for _, dim := range Dimensions {
		if schemaForDim(dim.Name) == nil {
			t.Errorf("missing schema for dim %q", dim.Name)
		}
	}
}

// max is a stdlib builtin in Go 1.21+, but be defensive for older toolchains.
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
