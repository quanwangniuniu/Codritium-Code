package problems

import (
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"codritium/backend/internal/auth"
	"codritium/backend/internal/platform/httpx"
)

// Handler serves the problem catalog.
type Handler struct{ Store Store }

func (h Handler) Routes(rt *httpx.Router) {
	rt.Handle("GET /api/problems", h.list)
	rt.Handle("GET /api/problems/search", h.search)
	rt.Handle("GET /api/problems/facets", h.facets)
	rt.Handle("GET /api/problems/{slug}", h.get)
}

// list returns catalog metadata. Only admins may list a status other than
// published, so candidates never see drafts or disabled rows via ?status=.
func (h Handler) list(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	if status == "" || !auth.FromContext(r.Context()).IsAdmin() {
		status = StatusPublished
	}
	out, err := h.Store.List(r.Context(), status)
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

// Search page size bounds.
const (
	defaultSearchLimit = 50
	maxSearchLimit     = 100
)

// search returns one filtered page of the catalog with the caller's progress
// on each row. The list page scrolls through it with ?offset=next_offset.
// Like list, only admins may ask for a status other than published.
func (h Handler) search(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	u := auth.FromContext(r.Context())
	sp := SearchParams{
		Status:     q.Get("status"),
		Category:   q.Get("category"),
		Difficulty: q.Get("difficulty"),
		Tag:        q.Get("tag"),
		UserStatus: q.Get("user_status"),
		Query:      strings.TrimSpace(q.Get("q")),
		Limit:      defaultSearchLimit,
	}
	if sp.Status == "" || !u.IsAdmin() {
		sp.Status = StatusPublished
	}
	if u != nil {
		sp.UserID = u.ID.String()
	}
	if sp.UserStatus != "" && !slices.Contains([]string{UserStatusTodo, UserStatusAttempted, UserStatusSolved}, sp.UserStatus) {
		httpx.BadRequest(w, "bad_user_status", "user_status must be todo, attempted or solved")
		return
	}
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			httpx.BadRequest(w, "bad_limit", "limit must be a positive integer")
			return
		}
		sp.Limit = min(n, maxSearchLimit)
	}
	if v := q.Get("offset"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			httpx.BadRequest(w, "bad_offset", "offset must be a non-negative integer")
			return
		}
		sp.Offset = n
	}
	page, err := h.Store.Search(r.Context(), sp)
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, page)
}

// facets returns the tag and category counts shown above the list, and how
// many problems the caller has solved.
func (h Handler) facets(w http.ResponseWriter, r *http.Request) {
	userID := ""
	if u := auth.FromContext(r.Context()); u != nil {
		userID = u.ID.String()
	}
	f, err := h.Store.Facets(r.Context(), userID)
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, f)
}

// get returns one problem with starter files for ?variant=as-is|stripped
// and the sample test bundle. The hidden test never leaves the server.
func (h Handler) get(w http.ResponseWriter, r *http.Request) {
	variant := r.URL.Query().Get("variant")
	if variant == "" {
		variant = VariantAsIs
	}
	if variant != VariantAsIs && variant != VariantStripped {
		httpx.BadRequest(w, "bad_variant", "variant must be as-is or stripped")
		return
	}
	p, err := h.Store.Get(r.Context(), r.PathValue("slug"))
	if errors.Is(err, ErrNotFound) {
		httpx.NotFound(w, "Problem not found.")
		return
	}
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	switch CanOpen(auth.FromContext(r.Context()), p) {
	case AccessNotFound:
		httpx.NotFound(w, "Problem not found.")
		return
	case AccessAuthRequired:
		httpx.Error(w, http.StatusUnauthorized, "auth_required", "Sign in to open this problem.")
		return
	case AccessProRequired:
		httpx.ErrorWith(w, http.StatusForbidden, "pro_required", "This problem needs a Pro plan.",
			map[string]any{"category": p.Category})
		return
	}
	if !p.VariantAllowed(variant) {
		httpx.BadRequest(w, "variant_unavailable", "That variant is not available for this problem.")
		return
	}

	resp := map[string]any{
		"slug":          p.Slug,
		"title":         p.Title,
		"category":      p.Category,
		"difficulty":    p.Difficulty,
		"readme_md":     p.ReadmeMD,
		"variant":       variant,
		"strip_variant": p.StripVariant,
		"starter_files": p.Starter.Variant(variant),
		"tags":          p.Tags,
		"status":        p.Status,
	}
	if p.SampleTestFilename != nil && p.SampleTestContent != nil {
		resp["sample_test_filename"] = *p.SampleTestFilename
		resp["sample_test_content"] = *p.SampleTestContent
	}
	httpx.JSON(w, http.StatusOK, resp)
}
