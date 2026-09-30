package auth

import "codritium/backend/internal/platform/httpx"

// Routes is the sign-in API: password, Google, GitHub, and email magic
// link. These run without the session middleware.
type Routes struct {
	Password   *PasswordAuthHandler
	Google     *GoogleHandler
	GitHub     *GitHubHandler
	Email      *EmailHandler
	CookieOpts CookieOptions
}

func (a Routes) Routes(rt *httpx.Router) {
	rt.Bare("POST /api/auth/register", a.Password.Register)
	rt.Bare("POST /api/auth/login", a.Password.Login)
	rt.Bare("POST /api/auth/logout", Logout(a.CookieOpts))
	rt.Bare("POST /api/auth/google/start", a.Google.Start)
	rt.Bare("GET /api/auth/google/callback", a.Google.Callback)
	rt.Bare("POST /api/auth/github/start", a.GitHub.Start)
	rt.Bare("GET /api/auth/github/callback", a.GitHub.Callback)
	rt.Bare("POST /api/auth/email/request", a.Email.Request)
	rt.Bare("GET /api/auth/email/verify", a.Email.Verify)
}
