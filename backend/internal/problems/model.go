package problems

import (
	"encoding/json"
	"errors"
)

// ErrNotFound means no problem has that slug.
var ErrNotFound = errors.New("problem not found")

// Status values operators set on problems.status.
const (
	StatusPublished = "published"
	StatusDraft     = "draft"
	StatusDisabled  = "disabled"
)

// CategoryCompanyPremium problems need a paid tier.
const CategoryCompanyPremium = "company_premium"

// Starter file variants.
const (
	VariantAsIs     = "as-is"
	VariantStripped = "stripped"
)

// Brief is a catalog row.
type Brief struct {
	Slug        string   `json:"slug"`
	Title       string   `json:"title"`
	Category    string   `json:"category"`
	Difficulty  string   `json:"difficulty"`
	Tags        []string `json:"tags"`
	Status      string   `json:"status"`
	RequiresPro bool     `json:"requires_pro"`
}

// Problem is one full problem row. HiddenTest* never leave the server.
type Problem struct {
	ID                 string
	Slug               string
	Title              string
	Category           string
	Difficulty         string
	ReadmeMD           string
	StripVariant       string
	Status             string
	Tags               []string
	Starter            StarterFiles
	SampleTestFilename *string
	SampleTestContent  *string
	HiddenTestFile     string
	HiddenTestContent  string
}

// RequiresPro reports whether the problem is behind the paid tier.
func (p *Problem) RequiresPro() bool { return p.Category == CategoryCompanyPremium }

// StarterFiles maps filename -> variant -> content, as stored in the seed.
type StarterFiles map[string]map[string]string

// ParseStarterFiles decodes the starter_files JSON column.
func ParseStarterFiles(raw []byte) (StarterFiles, error) {
	var sf StarterFiles
	if len(raw) == 0 {
		return StarterFiles{}, nil
	}
	if err := json.Unmarshal(raw, &sf); err != nil {
		return nil, err
	}
	return sf, nil
}

// Variant flattens to filename -> content for one variant, falling back to
// "as-is" for files that lack the requested variant. This was written out
// three times (problems handler, submissions, agent factory).
func (sf StarterFiles) Variant(variant string) map[string]string {
	out := make(map[string]string, len(sf))
	for name, variants := range sf {
		if content, ok := variants[variant]; ok {
			out[name] = content
		} else if content, ok := variants[VariantAsIs]; ok {
			out[name] = content
		}
	}
	return out
}

// VariantAllowed applies the problem's strip_variant policy.
func (p *Problem) VariantAllowed(variant string) bool {
	switch {
	case p.StripVariant == "as-is-only" && variant == VariantStripped:
		return false
	case p.StripVariant == "stripped-only" && variant == VariantAsIs:
		return false
	}
	return variant == VariantAsIs || variant == VariantStripped
}
