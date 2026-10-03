package problems

import "codritium/backend/internal/auth"

// Access is why a user may not open a problem.
type Access int

const (
	AccessOK Access = iota
	AccessNotFound
	AccessAuthRequired
	AccessProRequired
)

// CanOpen applies visibility rules: disabled problems are invisible, drafts
// are admin-only, and company-premium needs a pro or max tier.
func CanOpen(u *auth.User, p *Problem) Access {
	switch p.Status {
	case StatusDisabled:
		return AccessNotFound
	case StatusDraft:
		if !u.IsAdmin() {
			return AccessNotFound
		}
	}
	if p.RequiresPro() {
		if u == nil {
			return AccessAuthRequired
		}
		if u.Tier != "pro" && u.Tier != "max" {
			return AccessProRequired
		}
	}
	return AccessOK
}
