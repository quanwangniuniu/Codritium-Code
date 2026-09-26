package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
)

// GitHubHandler implements the GitHub OAuth code flow with PKCE per
// login-tech §4 Q3=A. The verifier is stored in oauth_states.code_verifier
// alongside the state token and presented on the token-exchange call.
type GitHubHandler struct {
	config     *oauth2.Config
	stateStore OAuthStateStore
	userStore  UserStore
	session    SessionManager
}

func NewGitHubHandler(clientID, clientSecret, redirectURL string, states OAuthStateStore, users UserStore, sess SessionManager) *GitHubHandler {
	return &GitHubHandler{
		config: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Scopes:       []string{"read:user", "user:email"},
			Endpoint:     github.Endpoint,
		},
		stateStore: states,
		userStore:  users,
		session:    sess,
	}
}

// Start: POST /api/auth/github/start
func (h *GitHubHandler) Start(w http.ResponseWriter, r *http.Request) {
	if h.config.ClientID == "" || h.config.ClientSecret == "" {
		http.Error(w, "github oauth not configured", http.StatusServiceUnavailable)
		return
	}
	state, err := GenerateState()
	if err != nil {
		http.Error(w, "csrf gen failed", http.StatusInternalServerError)
		return
	}
	verifier := GeneratePKCEVerifier()
	if err := h.stateStore.Insert(r.Context(), state, ProviderGitHub, verifier, OAuthStateExpiry); err != nil {
		http.Error(w, "state store failed", http.StatusInternalServerError)
		return
	}
	url := h.config.AuthCodeURL(state, oauth2.S256ChallengeOption(verifier))
	http.Redirect(w, r, url, http.StatusFound)
}

type githubProfile struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
}

type githubEmail struct {
	Email    string `json:"email"`
	Primary  bool   `json:"primary"`
	Verified bool   `json:"verified"`
}

// Callback: GET /api/auth/github/callback?state=&code=
func (h *GitHubHandler) Callback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	state := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")
	if state == "" || code == "" {
		http.Error(w, "missing state or code", http.StatusBadRequest)
		return
	}
	verifier, ok, err := h.stateStore.Consume(ctx, state, ProviderGitHub)
	if err != nil || !ok {
		http.Error(w, "invalid state", http.StatusBadRequest)
		return
	}
	token, err := h.config.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		http.Error(w, "token exchange failed", http.StatusBadGateway)
		return
	}
	profile, primaryEmail, err := h.fetchProfile(ctx, token)
	if err != nil || primaryEmail == "" {
		http.Error(w, "userinfo fetch failed", http.StatusBadGateway)
		return
	}
	providerUserID := strconv.FormatInt(profile.ID, 10)
	userID, err := h.userStore.UpsertFromOAuth(ctx, primaryEmail, profile.Name, profile.AvatarURL, ProviderGitHub, providerUserID)
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

func (h *GitHubHandler) fetchProfile(ctx context.Context, token *oauth2.Token) (*githubProfile, string, error) {
	client := h.config.Client(ctx, token)

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user", nil)
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("github /user returned %d", resp.StatusCode)
	}
	var p githubProfile
	if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
		return nil, "", err
	}
	if p.Email != "" {
		return &p, p.Email, nil
	}

	// /user gives no email when the user hides theirs in profile settings.
	// Fall back to the /user/emails endpoint, which the user:email scope
	// covers, and pick the verified primary.
	req2, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user/emails", nil)
	req2.Header.Set("Accept", "application/json")
	resp2, err := client.Do(req2)
	if err != nil {
		return &p, "", err
	}
	defer resp2.Body.Close()
	var emails []githubEmail
	if err := json.NewDecoder(resp2.Body).Decode(&emails); err != nil {
		return &p, "", err
	}
	for _, e := range emails {
		if e.Primary && e.Verified {
			return &p, e.Email, nil
		}
	}
	return &p, "", errors.New("no verified primary email at GitHub")
}
