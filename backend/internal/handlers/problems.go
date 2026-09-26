package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"codritium/backend/internal/auth"
)

type ProblemDeps struct {
	Pool *pgxpool.Pool
}

// ListProblems returns brief metadata for the catalog. Defaults to
// status='published'; non-admin callers cannot override the filter, so a
// candidate can never list draft or disabled rows even by passing ?status=.
func ListProblems(deps ProblemDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		statusFilter := r.URL.Query().Get("status")
		if statusFilter == "" {
			statusFilter = "published"
		}
		u := auth.FromContext(r.Context())
		if statusFilter != "published" && (u == nil || u.Role != "admin") {
			statusFilter = "published"
		}

		rows, err := deps.Pool.Query(r.Context(), `
			SELECT slug, title, category, difficulty, COALESCE(tags, '{}'), status
			FROM problems
			WHERE status = $1
			ORDER BY difficulty, slug`, statusFilter)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		defer rows.Close()

		type item struct {
			Slug         string   `json:"slug"`
			Title        string   `json:"title"`
			Category     string   `json:"category"`
			Difficulty   string   `json:"difficulty"`
			Tags         []string `json:"tags"`
			Status       string   `json:"status"`
			RequiresPro  bool     `json:"requires_pro"`
		}
		out := []item{}
		for rows.Next() {
			var it item
			if err := rows.Scan(&it.Slug, &it.Title, &it.Category, &it.Difficulty, &it.Tags, &it.Status); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
				return
			}
			it.RequiresPro = it.Category == "company_premium"
			out = append(out, it)
		}
		writeJSON(w, http.StatusOK, out)
	}
}

// GetProblem returns one problem with starter files for the requested variant
// and the sample_test bundle. The hidden test stays server-side. Querying
// a company_premium problem requires a pro or max tier; standard users get
// 403 with reason=pro_required so the frontend can prompt an upgrade.
// Query param: ?variant=as-is (default) or ?variant=stripped
func GetProblem(deps ProblemDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slug := r.PathValue("slug")
		if slug == "" {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "slug required"})
			return
		}
		variant := r.URL.Query().Get("variant")
		if variant == "" {
			variant = "as-is"
		}
		if variant != "as-is" && variant != "stripped" {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "variant must be as-is or stripped"})
			return
		}

		var (
			title, category, difficulty, readmeMD, stripVariant, status string
			starterJSON                                                  []byte
			tags                                                         []string
			sampleTestFilename, sampleTestContent                        *string
		)
		err := deps.Pool.QueryRow(r.Context(), `
			SELECT title, category, difficulty, readme_md, starter_files::text, strip_variant,
			       COALESCE(tags, '{}'), status,
			       sample_test_filename, sample_test_content
			FROM problems WHERE slug = $1`, slug,
		).Scan(&title, &category, &difficulty, &readmeMD, &starterJSON, &stripVariant,
			&tags, &status,
			&sampleTestFilename, &sampleTestContent)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "problem not found"})
			return
		}

		// Operator-disabled problems are invisible even by direct slug lookup.
		// Drafts are visible to admins only.
		u := auth.FromContext(r.Context())
		switch status {
		case "disabled":
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "problem not found"})
			return
		case "draft":
			if u == nil || u.Role != "admin" {
				writeJSON(w, http.StatusNotFound, map[string]any{"error": "problem not found"})
				return
			}
		}

		// Company-premium tier gate.
		if category == "company_premium" {
			if u == nil {
				writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "auth_required"})
				return
			}
			if u.Tier != "pro" && u.Tier != "max" {
				writeJSON(w, http.StatusForbidden, map[string]any{
					"error":    "pro_required",
					"category": category,
				})
				return
			}
		}

		// Strip_variant policy: if 'as-is-only' and variant=stripped, reject;
		// if 'stripped-only' and variant=as-is, reject.
		if (stripVariant == "as-is-only" && variant == "stripped") ||
			(stripVariant == "stripped-only" && variant == "as-is") {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "variant not available for this problem"})
			return
		}

		// Decode starter files and select the requested variant; fall back to
		// as-is when the requested variant is missing for a given file.
		var allFiles map[string]map[string]string
		if err := json.Unmarshal(starterJSON, &allFiles); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "decode starter files: " + err.Error()})
			return
		}
		picked := map[string]string{}
		for name, variants := range allFiles {
			if content, ok := variants[variant]; ok {
				picked[name] = content
			} else if content, ok := variants["as-is"]; ok {
				picked[name] = content
			}
		}

		resp := map[string]any{
			"slug":          slug,
			"title":         title,
			"category":      category,
			"difficulty":    difficulty,
			"readme_md":     readmeMD,
			"variant":       variant,
			"strip_variant": stripVariant,
			"starter_files": picked,
			"tags":          tags,
			"status":        status,
		}
		if sampleTestFilename != nil && sampleTestContent != nil {
			resp["sample_test_filename"] = *sampleTestFilename
			resp["sample_test_content"] = *sampleTestContent
		}
		writeJSON(w, http.StatusOK, resp)
	}
}
