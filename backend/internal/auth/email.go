package auth

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/mail"
	"net/smtp"
	"strings"
	"time"
)

// EmailHandler runs the magic-link sign-in flow per login-tech §5. The
// request endpoint issues a 24h one-use token and 60s rate-limited delivery;
// the verify endpoint atomically consumes the token and writes a session.
type EmailHandler struct {
	smtpHost   string
	smtpPort   int
	smtpUser   string
	smtpPass   string
	smtpFrom   string
	siteURL    string
	tokenStore MagicLinkTokenStore
	userStore  UserStore
	session    SessionManager
}

func NewEmailHandler(smtpHost string, smtpPort int, smtpUser, smtpPass, smtpFrom, siteURL string, tokens MagicLinkTokenStore, users UserStore, sess SessionManager) *EmailHandler {
	return &EmailHandler{
		smtpHost:   smtpHost,
		smtpPort:   smtpPort,
		smtpUser:   smtpUser,
		smtpPass:   smtpPass,
		smtpFrom:   smtpFrom,
		siteURL:    siteURL,
		tokenStore: tokens,
		userStore:  users,
		session:    sess,
	}
}

type emailRequestBody struct {
	Email string `json:"email"`
}

// Request: POST /api/auth/email/request {"email":"..."}
func (h *EmailHandler) Request(w http.ResponseWriter, r *http.Request) {
	if h.smtpHost == "" || h.smtpFrom == "" {
		http.Error(w, "email login not configured", http.StatusServiceUnavailable)
		return
	}
	ctx := r.Context()
	var body emailRequestBody
	if r.Body == nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if _, err := mail.ParseAddress(body.Email); err != nil {
		http.Error(w, "invalid email", http.StatusBadRequest)
		return
	}
	email := strings.ToLower(strings.TrimSpace(body.Email))

	lastAt, err := h.tokenStore.LastRequestedAt(ctx, email)
	if err == nil && !lastAt.IsZero() && time.Since(lastAt) < MagicLinkRateLimit {
		http.Error(w, "too many requests; try again in a minute", http.StatusTooManyRequests)
		return
	}

	token, err := generateMagicToken()
	if err != nil {
		http.Error(w, "token gen failed", http.StatusInternalServerError)
		return
	}
	expiresAt := time.Now().Add(MagicLinkTTL)
	if err := h.tokenStore.Insert(ctx, token, email, expiresAt); err != nil {
		http.Error(w, "token store failed", http.StatusInternalServerError)
		return
	}
	if err := h.sendEmail(email, token); err != nil {
		http.Error(w, "email send failed", http.StatusBadGateway)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Verify: GET /api/auth/email/verify?token=…
func (h *EmailHandler) Verify(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "missing token", http.StatusBadRequest)
		return
	}
	email, ok, err := h.tokenStore.Consume(ctx, token)
	if err != nil || !ok {
		http.Error(w, "link expired or already used", http.StatusGone)
		return
	}
	userID, err := h.userStore.UpsertFromEmail(ctx, email)
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

func generateMagicToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

const emailTemplateText = `Sign in to Codritium

Click the link below to finish signing in:

  {{.URL}}

This link expires in 24 hours and can only be used once.
If you didn't request this, you can safely ignore the message.

— Codritium team
`

var emailTmpl = template.Must(template.New("magic").Parse(emailTemplateText))

func (h *EmailHandler) sendEmail(to, token string) error {
	var buf bytes.Buffer
	data := struct{ URL string }{
		URL: fmt.Sprintf("%s/api/auth/email/verify?token=%s", strings.TrimRight(h.siteURL, "/"), token),
	}
	if err := emailTmpl.Execute(&buf, data); err != nil {
		return err
	}
	msg := []byte(fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: Sign in to Codritium\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n%s",
		h.smtpFrom, to, buf.String(),
	))
	addr := fmt.Sprintf("%s:%d", h.smtpHost, h.smtpPort)
	auth := smtp.PlainAuth("", h.smtpUser, h.smtpPass, h.smtpHost)
	return smtp.SendMail(addr, auth, h.smtpFrom, []string{to}, msg)
}
