package forum

import (
	"regexp"
	"time"

	"github.com/google/uuid"
)

// Sections follow LeetCode Discuss. "interview" is Interview Experience.
var forumSections = map[string]bool{
	"interview-question": true,
	"interview":          true,
	"compensation":       true,
	"career":             true,
	"study-guide":        true,
	"general":            true,
	"feedback":           true,
}

// Sections where posts (and comments on them) may be anonymous.
var forumAnonSections = map[string]bool{
	"interview-question": true,
	"interview":          true,
	"compensation":       true,
}

const (
	forumTitleMin    = 5
	forumTitleMax    = 150
	forumBodyMax     = 20000
	forumCommentMax  = 5000
	forumMaxTags     = 5
	forumExcerptLen  = 200
	forumPageDefault = 20
	forumPageMax     = 50
	forumOffsetMax   = 2000
	forumPinnedMax   = 5
	forumTrendingMax = 10

	// Per-user write limits (admins are exempt).
	forumPostsPerHour    = 5
	forumCommentsPerHour = 30
	forumVotesPerMinute  = 120
	forumReportsPerHour  = 10
	forumImagesPerHour   = 20
)

var forumTagPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9+#.-]{0,23}$`)

type forumAuthor struct {
	ID          uuid.UUID `json:"id"`
	Handle      string    `json:"handle"`
	DisplayName string    `json:"display_name"`
	AvatarURL   string    `json:"avatar_url"`
	AvatarColor string    `json:"avatar_color"`
	// Verified marks official (admin) accounts, like LeetCode's check badge.
	Verified bool `json:"verified"`
	// Reputation is upvotes received on the author's named posts and comments.
	Reputation int `json:"reputation"`
}

type forumPost struct {
	ID           uuid.UUID    `json:"id"`
	Section      string       `json:"section"`
	ProblemSlug  *string      `json:"problem_slug"`
	Title        string       `json:"title"`
	Excerpt      string       `json:"excerpt"`
	BodyMD       string       `json:"body_md,omitempty"`
	Tags         []string     `json:"tags"`
	IsAnonymous  bool         `json:"is_anonymous"`
	IsPinned     bool         `json:"is_pinned"`
	IsLocked     bool         `json:"is_locked"`
	Author       *forumAuthor `json:"author"`
	IsMine       bool         `json:"is_mine"`
	Upvotes      int          `json:"upvotes"`
	Downvotes    int          `json:"downvotes"`
	Score        int          `json:"score"`
	ViewCount    int          `json:"view_count"`
	CommentCount int          `json:"comment_count"`
	MyVote       int          `json:"my_vote"`
	IsBookmarked bool         `json:"is_bookmarked"`
	IsFollowing  bool         `json:"is_following"`
	CreatedAt    time.Time    `json:"created_at"`
	UpdatedAt    time.Time    `json:"updated_at"`
}

type forumComment struct {
	ID          uuid.UUID    `json:"id"`
	PostID      uuid.UUID    `json:"post_id"`
	ParentID    *uuid.UUID   `json:"parent_id"`
	Body        string       `json:"body"`
	IsAnonymous bool         `json:"is_anonymous"`
	Author      *forumAuthor `json:"author"`
	IsMine      bool         `json:"is_mine"`
	IsOP        bool         `json:"is_op"`
	Upvotes     int          `json:"upvotes"`
	Downvotes   int          `json:"downvotes"`
	Score       int          `json:"score"`
	MyVote      int          `json:"my_vote"`
	CreatedAt   time.Time    `json:"created_at"`
	EditedAt    *time.Time   `json:"edited_at"`
	// IsDeleted marks a deleted top-level comment kept as a "[deleted]"
	// placeholder because it still has replies. Its body and author are blank.
	IsDeleted bool           `json:"is_deleted"`
	Replies   []forumComment `json:"replies,omitempty"`
}

type forumTrendingItem struct {
	ID        uuid.UUID `json:"id"`
	Title     string    `json:"title"`
	Section   string    `json:"section"`
	Tag       string    `json:"tag"`
	ViewCount int       `json:"view_count"`
}
