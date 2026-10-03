package httpx

import "net/http"

// Router is what modules register routes on. Handle routes run behind the
// session middleware (the user is attached when signed in; handlers decide
// whether anonymous is allowed). Bare routes skip the session lookup.
type Router struct {
	mux     *http.ServeMux
	session func(http.Handler) http.Handler
}

// NewRouter wraps mux; session attaches the signed-in user to the context.
func NewRouter(mux *http.ServeMux, session func(http.Handler) http.Handler) *Router {
	return &Router{mux: mux, session: session}
}

// Handle registers a session-aware route.
func (rt *Router) Handle(pattern string, h http.HandlerFunc) {
	rt.mux.Handle(pattern, rt.session(h))
}

// Bare registers a route without the session lookup.
func (rt *Router) Bare(pattern string, h http.HandlerFunc) {
	rt.mux.Handle(pattern, h)
}

// Module is implemented by every feature module.
type Module interface {
	Routes(rt *Router)
}
