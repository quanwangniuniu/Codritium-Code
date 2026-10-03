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

// Tags is the fixed topic vocabulary a problem's tags are drawn from. The
// catalog page lists them as filters, so a typo in a seed file would show up
// as a stray one-problem topic; the seed test rejects anything not listed.
var Tags = []string{
	"Access Control", "Architecture", "Async", "Auth & Tokens", "Billing & Payments",
	"Caching", "Cloud Infra", "Concurrency", "Configuration", "Cryptography",
	"Data Processing", "Data Sync", "Date & Time", "Idempotency", "Injection",
	"LLM APIs", "Media & Files", "Messaging", "Multi-tenancy", "Numeric Logic",
	"Observability", "Pagination", "Resilience", "Retries", "State Machines",
	"Streaming", "Text & Encoding", "Validation", "Web Security", "Webhooks",
}

// Per-user progress on a problem.
const (
	UserStatusTodo      = "todo"
	UserStatusAttempted = "attempted"
	UserStatusSolved    = "solved"
)

// SearchParams filters and pages the catalog. Empty fields do not filter.
type SearchParams struct {
	Status     string // problems.status; required
	Category   string
	Difficulty string
	Tag        string
	UserStatus string // one of the UserStatus* values
	Query      string // case-insensitive title substring
	UserID     string // "" for anonymous: every problem is then todo
	Limit      int
	Offset     int
}

// SearchItem is a catalog row plus the caller's progress on it.
type SearchItem struct {
	Brief
	UserStatus string `json:"user_status"`
}

// SearchPage is one page of search results. NextOffset is nil on the last page.
type SearchPage struct {
	Items      []SearchItem `json:"items"`
	Total      int          `json:"total"`
	NextOffset *int         `json:"next_offset"`
}

// FacetCount is one filter value and how many published problems carry it.
type FacetCount struct {
	Value string `json:"value"`
	Count int    `json:"count"`
}

// Facets are the catalog-wide counts the list page renders above the results.
type Facets struct {
	Total      int          `json:"total"`
	Solved     int          `json:"solved"`
	Tags       []FacetCount `json:"tags"`
	Categories []FacetCount `json:"categories"`
}
