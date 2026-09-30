package problems

import (
	"errors"
	"net/http"

	"codritium/backend/internal/auth"
	"codritium/backend/internal/platform/httpx"
)

// Handler serves the problem catalog.
type Handler struct{ Store Store }

func (h Handler) Routes(rt *httpx.Router) {
	rt.Handle("GET /api/problems", h.list)
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
