package forum

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"unicode/utf8"

	"codritium/backend/internal/platform/httpx"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"codritium/backend/internal/auth"
)

func forumError(w http.ResponseWriter, status int, code string) {
	httpx.Error(w, status, code, forumMessages[code])
}

// forumMessages are the human texts for forum error codes.
var forumMessages = map[string]string{
	"bad_json":              "Request body is not valid JSON.",
	"invalid_section":       "Unknown forum section.",
	"invalid_title":         "Title must be 1-150 characters.",
	"invalid_body":          "Post body is empty or too long.",
	"invalid_tag":           "Tags use lowercase letters, digits, and + # . -",
	"too_many_tags":         "Too many tags.",
	"unknown_problem":       "That problem does not exist.",
	"anonymous_not_allowed": "Anonymous posting is not allowed in this section.",
	"not_found":             "Not found.",
	"forbidden":             "You can't do that.",
	"login_required":        "Sign in to continue.",
}

func viewerID(u *auth.User) uuid.UUID {
	if u == nil {
		return uuid.Nil
	}
	return u.ID
}

// canSeeAuthor reports whether the viewer may see who wrote an item.
func canSeeAuthor(viewer *auth.User, authorID uuid.UUID, anonymous bool) bool {
	if !anonymous {
		return true
	}
	return viewer != nil && (viewer.ID == authorID || viewer.Role == "admin")
}

// Column list shared by every post query; bodyExpr picks the full body or a
// prefix long enough to build an excerpt from.
func forumPostColumns(bodyExpr string) string {
	return `p.id, p.user_id, p.section, p.problem_slug, p.title, ` + bodyExpr + `,
	        p.tags, p.is_anonymous, p.is_pinned, p.upvotes, p.downvotes,
	        p.view_count, p.comment_count, p.created_at, p.updated_at,
	        u.handle, u.display_name, COALESCE(u.avatar_url,''), COALESCE(u.avatar_color,''),
	        COALESCE(u.role,''), COALESCE(v.value, 0)`
}

// forumPostFrom joins author and the viewer's vote ($1 is the viewer id).
const forumPostFrom = `
	FROM forum_posts p
	JOIN users u ON u.id = p.user_id
	LEFT JOIN forum_post_votes v ON v.post_id = p.id AND v.user_id = $1`

func scanForumPost(row pgx.Row, viewer *auth.User, full bool) (forumPost, error) {
	var (
		p        forumPost
		authorID uuid.UUID
		body     string
		a        forumAuthor
		role     string
	)
	if err := row.Scan(&p.ID, &authorID, &p.Section, &p.ProblemSlug, &p.Title, &body,
		&p.Tags, &p.IsAnonymous, &p.IsPinned, &p.Upvotes, &p.Downvotes,
		&p.ViewCount, &p.CommentCount, &p.CreatedAt, &p.UpdatedAt,
		&a.Handle, &a.DisplayName, &a.AvatarURL, &a.AvatarColor, &role, &p.MyVote); err != nil {
		return forumPost{}, err
	}
	p.Score = p.Upvotes - p.Downvotes
	p.Excerpt = forumExcerpt(body)
	if full {
		p.BodyMD = body
	}
	if p.Tags == nil {
		p.Tags = []string{}
	}
	p.IsMine = viewer != nil && viewer.ID == authorID
	if canSeeAuthor(viewer, authorID, p.IsAnonymous) {
		a.ID = authorID
		a.Verified = role == "admin"
		p.Author = &a
	}
	return p, nil
}

func loadForumPost(ctx context.Context, pool *pgxpool.Pool, id uuid.UUID, viewer *auth.User) (forumPost, error) {
	row := pool.QueryRow(ctx, `SELECT `+forumPostColumns("p.body_md")+forumPostFrom+`
		WHERE p.id = $2 AND p.deleted_at IS NULL`, viewerID(viewer), id)
	return scanForumPost(row, viewer, true)
}

var (
	mdFence     = regexp.MustCompile("(?s)```.*?(```|$)")
	mdLink      = regexp.MustCompile(`!?\[([^\]]*)\]\([^)]*\)`)
	mdLineStart = regexp.MustCompile(`(?m)^\s{0,3}(#{1,6}\s+|>\s?|[-*+]\s+(\[[ xX]\]\s+)?|\d+\.\s+)`)
	mdTableRule = regexp.MustCompile(`(?m)^\s*\|?\s*:?-+:?\s*(\|\s*:?-+:?\s*)*\|?\s*$`)
	mdTablePipe = regexp.MustCompile(`\s*\|\s*`)
	mdEmphasis  = regexp.MustCompile("[*_`~]+")
	mdSpace     = regexp.MustCompile(`\s+`)
)

// forumExcerpt turns a markdown body into a short plain-text preview for
// feed cards.
func forumExcerpt(md string) string {
	s := mdFence.ReplaceAllString(md, " ")
	s = mdLink.ReplaceAllString(s, "$1")
	s = mdLineStart.ReplaceAllString(s, "")
	s = mdTableRule.ReplaceAllString(s, " ")
	s = mdTablePipe.ReplaceAllString(s, " ")
	s = mdEmphasis.ReplaceAllString(s, "")
	s = strings.TrimSpace(mdSpace.ReplaceAllString(s, " "))
	if utf8.RuneCountInString(s) <= forumExcerptLen {
		return s
	}
	r := []rune(s)
	return strings.TrimSpace(string(r[:forumExcerptLen])) + "…"
}

func normalizeForumTags(in []string) ([]string, error) {
	out := make([]string, 0, len(in))
	seen := map[string]bool{}
	for _, raw := range in {
		t := strings.ToLower(strings.Join(strings.Fields(raw), "-"))
		t = strings.TrimPrefix(t, "#")
		if t == "" || seen[t] {
			continue
		}
		if !forumTagPattern.MatchString(t) {
			return nil, fmt.Errorf("invalid_tag")
		}
		seen[t] = true
		out = append(out, t)
	}
	if len(out) > forumMaxTags {
		return nil, fmt.Errorf("too_many_tags")
	}
	return out, nil
}

func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}
