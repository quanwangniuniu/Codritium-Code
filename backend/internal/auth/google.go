package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// GoogleHandler implements the Google OAuth code flow per login-tech §3.
// Q2=B leaves AccessTypeOffline off: no refresh token, the user re-logs in
// when the session cookie expires.
type GoogleHandler struct {
	config     *oauth2.Config
	stateStore OAuthStateStore
	userStore  UserStore
	session    SessionManager
}

// NewGoogleHandler wires the four collaborators. Empty client config is
// permitted at construction so the server can still boot in dev without
// Google credentials; the handler returns 503 when invoked unconfigured.
func NewGoogleHandler(clientID, clientSecret, redirectURL string, states OAuthStateStore, users UserStore, sess SessionManager) *GoogleHandler {
	return &GoogleHandler{
		config: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Scopes:       []string{"openid", "email", "profile"},
			Endpoint:     google.Endpoint,
		},
		stateStore: states,
		userStore:  users,
		session:    sess,
	}
}

// Start: POST /api/auth/google/start
// Issues a redirect to the Google consent screen with a fresh state token.
// PKCE is intentionally skipped (Q3=A) — Google's confidential-client flow
// already protects the code exchange.
func (h *GoogleHandler) Start(w http.ResponseWriter, r *http.Request) {
	if h.config.ClientID == "" || h.config.ClientSecret == "" {
		http.Error(w, "google oauth not configured", http.StatusServiceUnavailable)
		return
	}
	state, err := GenerateState()
	if err != nil {
		http.Error(w, "csrf gen failed", http.StatusInternalServerError)
		return
	}
	if err := h.stateStore.Insert(r.Context(), state, ProviderGoogle, "", OAuthStateExpiry); err != nil {
		http.Error(w, "state store failed", http.StatusInternalServerError)
		return
	}
	url := h.config.AuthCodeURL(state)
	http.Redirect(w, r, url, http.StatusFound)
}

type googleProfile struct {
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
}

// Callback: GET /api/auth/google/callback?state=&code=
func (h *GoogleHandler) Callback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	state := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")
	if state == "" || code == "" {
		http.Error(w, "missing state or code", http.StatusBadRequest)
		return
	}
	if _, ok, err := h.stateStore.Consume(ctx, state, ProviderGoogle); err != nil || !ok {
		http.Error(w, "invalid state", http.StatusBadRequest)
		return
	}
	token, err := h.config.Exchange(ctx, code)
	if err != nil {
		http.Error(w, "token exchange failed", http.StatusBadGateway)
		return
	}
	profile, err := h.fetchProfile(ctx, token)
	if err != nil {
		http.Error(w, "userinfo fetch failed", http.StatusBadGateway)
		return
	}
	if !profile.EmailVerified {
		http.Error(w, "email not verified at provider", http.StatusForbidden)
		return
	}
	userID, err := h.userStore.UpsertFromOAuth(ctx, profile.Email, profile.Name, profile.Picture, ProviderGoogle, profile.Sub)
	if err != nil {
		http.Error(w, "user upsert failed", http.StatusInternalServerError)
		return
	}
	if err := h.session.Create(w, userID, 30*24*time.Hour); err != nil {
		http.Error(w, "session creation failed", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/dashboard", http.StatusFound)
}

func (h *GoogleHandler) fetchProfile(ctx context.Context, token *oauth2.Token) (*googleProfile, error) {
	client := h.config.Client(ctx, token)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://openidconnect.googleapis.com/v1/userinfo", nil)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("non-200 from userinfo endpoint")
	}
	var p googleProfile
	if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
		return nil, err
	}
	return &p, nil
}
