// Package account serves the signed-in user's own record: GET and
// PATCH /api/me, and the avatar endpoints.
package account

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"codritium/backend/internal/platform/httpx"

	"codritium/backend/internal/auth"
)

// details are the profile fields only the account pages read, so they are
// not part of auth.User (which is loaded on every request).
type details struct {
	Gender      string
	Birthday    string // YYYY-MM-DD, or "" when unset
	WebsiteURL  string
	GithubURL   string
	LinkedinURL string
	XURL        string
}

func loadDetails(ctx context.Context, pool *pgxpool.Pool, id uuid.UUID) (details, error) {
	var d details
	err := pool.QueryRow(ctx, `
		SELECT COALESCE(gender, ''), COALESCE(to_char(birthday, 'YYYY-MM-DD'), ''),
		       COALESCE(website_url, ''), COALESCE(github_url, ''),
		       COALESCE(linkedin_url, ''), COALESCE(x_url, '')
		FROM users WHERE id = $1`, id,
	).Scan(&d.Gender, &d.Birthday, &d.WebsiteURL, &d.GithubURL, &d.LinkedinURL, &d.XURL)
	return d, err
}

// writeMe writes the current user as JSON (the /api/me payload).
// is_pro and avatar_color stay in the payload so older frontend code keeps
// working while it migrates to tier and avatar_url.
func writeMe(w http.ResponseWriter, u *auth.User, d details) {
	httpx.JSON(w, http.StatusOK, map[string]any{
		"id":           u.ID,
		"handle":       u.Handle,
		"email":        u.Email,
		"display_name": u.DisplayName,
		"region":       u.Region,
		"tier":         u.Tier,
		"credits":      u.Credits,
		"role":         u.Role,
		"avatar_url":   u.AvatarURL,
		"avatar_color": u.AvatarColor,
		"bio":          u.Bio,
		"is_pro":       u.IsPro,
		"gender":       d.Gender,
		"birthday":     d.Birthday,
		"website_url":  d.WebsiteURL,
		"github_url":   d.GithubURL,
		"linkedin_url": d.LinkedinURL,
		"x_url":        d.XURL,
	})
}

// Routes registers the account API.
func (h Handler) Routes(rt *httpx.Router) {
	rt.Handle("GET /api/me", h.me)
	rt.Handle("PATCH /api/me", h.updateMe)
	rt.Handle("PUT /api/me/avatar", h.putAvatar)
	rt.Handle("DELETE /api/me/avatar", h.deleteAvatar)
	rt.Handle("GET /api/users/{id}/avatar", h.getAvatar)
}

// Handler serves /api/me.
type Handler struct {
	Pool *pgxpool.Pool
	// Now is injectable so the birthday check is testable; nil means time.Now.
	Now func() time.Time
}

func (h Handler) now() time.Time {
	if h.Now != nil {
		return h.Now()
	}
	return time.Now()
}

func (h Handler) me(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.Require(w, r)
	if !ok {
		return
	}
	d, err := loadDetails(r.Context(), h.Pool, u.ID)
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	writeMe(w, u, d)
}

// Profile field limits (in characters).
const (
	maxDisplayNameRunes = 50
	maxBioRunes         = 280
	maxRegionRunes      = 40
	maxURLBytes         = 200
)

var genders = []string{"male", "female", "non_binary", "other"}

type updateMeRequest struct {
	DisplayName *string `json:"display_name"`
	Bio         *string `json:"bio"`
	Region      *string `json:"region"`
	Gender      *string `json:"gender"`
	Birthday    *string `json:"birthday"`
	WebsiteURL  *string `json:"website_url"`
	GithubURL   *string `json:"github_url"`
	LinkedinURL *string `json:"linkedin_url"`
	XURL        *string `json:"x_url"`
}

// UpdateMe edits the signed-in user's profile fields. Omitted fields are
// left unchanged; an empty string clears an optional field. PATCH /api/me
func (h Handler) updateMe(w http.ResponseWriter, r *http.Request) {
	u := auth.FromContext(r.Context())
	if u == nil {
		httpx.JSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
		return
	}
	var req updateMeRequest
	if !httpx.DecodeWith(w, r, &req, httpx.DecodeOptions{MaxBytes: 8 << 10, Strict: true}) {
		return
	}
	d, err := loadDetails(r.Context(), h.Pool, u.ID)
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	next := *u
	err = applyProfileUpdate(&next, req)
	if err == nil {
		err = applyDetailsUpdate(&d, req, h.now())
	}
	if err != nil {
		httpx.BadRequest(w, err.Error(), fieldMessages[err.Error()])
		return
	}
	_, err = h.Pool.Exec(r.Context(), `
		UPDATE users SET display_name = $2, bio = NULLIF($3, ''), region = NULLIF($4, ''),
		       gender = NULLIF($5, ''), birthday = NULLIF($6, '')::date,
		       website_url = NULLIF($7, ''), github_url = NULLIF($8, ''),
		       linkedin_url = NULLIF($9, ''), x_url = NULLIF($10, '')
		WHERE id = $1`, u.ID, next.DisplayName, next.Bio, next.Region,
		d.Gender, d.Birthday, d.WebsiteURL, d.GithubURL, d.LinkedinURL, d.XURL)
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	writeMe(w, &next, d)
}

func applyProfileUpdate(u *auth.User, req updateMeRequest) error {
	if req.DisplayName != nil {
		v := strings.TrimSpace(*req.DisplayName)
		if v == "" || utf8.RuneCountInString(v) > maxDisplayNameRunes {
			return errors.New("invalid_display_name")
		}
		u.DisplayName = v
	}
	if req.Bio != nil {
		v := strings.TrimSpace(*req.Bio)
		if utf8.RuneCountInString(v) > maxBioRunes {
			return errors.New("invalid_bio")
		}
		u.Bio = v
	}
	if req.Region != nil {
		v := strings.TrimSpace(*req.Region)
		if utf8.RuneCountInString(v) > maxRegionRunes {
			return errors.New("invalid_region")
		}
		u.Region = v
	}
	return nil
}

func applyDetailsUpdate(d *details, req updateMeRequest, now time.Time) error {
	if req.Gender != nil {
		v := strings.TrimSpace(*req.Gender)
		if v != "" && !contains(genders, v) {
			return errors.New("invalid_gender")
		}
		d.Gender = v
	}
	if req.Birthday != nil {
		v := strings.TrimSpace(*req.Birthday)
		if v != "" {
			day, err := time.Parse("2006-01-02", v)
			if err != nil || day.Year() < 1900 || day.After(now) {
				return errors.New("invalid_birthday")
			}
		}
		d.Birthday = v
	}
	links := []struct {
		in   *string
		out  *string
		kind linkKind
		code string
	}{
		{req.WebsiteURL, &d.WebsiteURL, linkWebsite, "invalid_website_url"},
		{req.GithubURL, &d.GithubURL, linkGithub, "invalid_github_url"},
		{req.LinkedinURL, &d.LinkedinURL, linkLinkedin, "invalid_linkedin_url"},
		{req.XURL, &d.XURL, linkX, "invalid_x_url"},
	}
	for _, l := range links {
		if l.in == nil {
			continue
		}
		v, ok := normalizeLink(l.kind, *l.in)
		if !ok {
			return errors.New(l.code)
		}
		*l.out = v
	}
	return nil
}

func contains(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

// linkKind says which site a profile link must point at.
type linkKind struct {
	// hosts the URL may use (after dropping "www."); empty means any host.
	hosts []string
	// profileBase turns a bare username into a URL; "" means bare names are
	// not accepted.
	profileBase string
}

var (
	linkWebsite  = linkKind{}
	linkGithub   = linkKind{hosts: []string{"github.com"}, profileBase: "https://github.com/"}
	linkLinkedin = linkKind{hosts: []string{"linkedin.com"}, profileBase: "https://www.linkedin.com/in/"}
	linkX        = linkKind{hosts: []string{"x.com", "twitter.com"}, profileBase: "https://x.com/"}
)

var bareUsername = regexp.MustCompile(`^@?[A-Za-z0-9][A-Za-z0-9_.-]*$`)

// normalizeLink validates a profile link and returns the form to store.
// People paste these in every shape, so a missing scheme is filled in and,
// for the social sites, a bare username becomes the profile URL. "" clears.
func normalizeLink(kind linkKind, raw string) (string, bool) {
	v := strings.TrimSpace(raw)
	if v == "" {
		return "", true
	}
	if kind.profileBase != "" && !strings.Contains(v, ".") && bareUsername.MatchString(v) {
		v = kind.profileBase + strings.TrimPrefix(v, "@")
	} else if !strings.Contains(v, "://") {
		v = "https://" + v
	}
	if len(v) > maxURLBytes {
		return "", false
	}
	u, err := url.Parse(v)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil {
		return "", false
	}
	host := strings.ToLower(u.Hostname())
	if !strings.Contains(host, ".") {
		return "", false
	}
	if len(kind.hosts) > 0 {
		base := strings.TrimPrefix(host, "www.")
		ok := false
		for _, h := range kind.hosts {
			if base == h || strings.HasSuffix(base, "."+h) {
				ok = true
			}
		}
		// A link to the site's front page is not a profile.
		if !ok || strings.Trim(u.Path, "/") == "" {
			return "", false
		}
	}
	return u.String(), true
}

var fieldMessages = map[string]string{
	"invalid_display_name": "Display name must be 1-50 characters.",
	"invalid_bio":          "Bio must be at most 280 characters.",
	"invalid_region":       "Location must be at most 40 characters.",
	"invalid_gender":       "Pick one of the listed options.",
	"invalid_birthday":     "Enter a real date that is not in the future.",
	"invalid_website_url":  "Enter a valid web address.",
	"invalid_github_url":   "Enter your GitHub username or profile link.",
	"invalid_linkedin_url": "Enter your LinkedIn username or profile link.",
	"invalid_x_url":        "Enter your X username or profile link.",
}
