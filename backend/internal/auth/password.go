package auth

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/mail"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

const (
	minimumPasswordRunes = 8
	maximumPasswordBytes = 72 // bcrypt's input limit
	maximumAuthBodyBytes = 8 << 10
)

var errEmailAlreadyRegistered = errors.New("email already registered")

// A random hash lets failed logins perform a bcrypt comparison even when the
// email is unknown, reducing account-enumeration timing differences.
var dummyPasswordHash = func() []byte {
	hash, err := bcrypt.GenerateFromPassword([]byte("not-a-real-account-password"), bcrypt.DefaultCost)
	if err != nil {
		panic(err)
	}
	return hash
}()

type PasswordAuthHandler struct {
	pool         *pgxpool.Pool
	cookieSecret string
	cookieOpts   CookieOptions
}

func NewPasswordAuthHandler(pool *pgxpool.Pool, cookieSecret string, cookieOpts CookieOptions) *PasswordAuthHandler {
	return &PasswordAuthHandler{pool: pool, cookieSecret: cookieSecret, cookieOpts: cookieOpts}
}

type passwordCredentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Register creates a user and starts a session. POST /api/auth/register.
func (h *PasswordAuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	credentials, ok := readCredentials(w, r)
	if !ok {
		return
	}
	email, err := normalizeEmail(credentials.Email)
	if err != nil || validatePassword(credentials.Password) != nil {
		http.Error(w, "invalid email or password", http.StatusBadRequest)
		return
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(credentials.Password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "could not create account", http.StatusInternalServerError)
		return
	}

	userID, err := createPasswordUser(r.Context(), h.pool, email, string(passwordHash))
	if errors.Is(err, errEmailAlreadyRegistered) {
		http.Error(w, "email already registered", http.StatusConflict)
		return
	}
	if err != nil {
		http.Error(w, "could not create account", http.StatusInternalServerError)
		return
	}

	SetSessionCookie(w, userID, h.cookieSecret, h.cookieOpts)
	w.WriteHeader(http.StatusCreated)
}

// Login verifies the submitted password and starts a session. POST /api/auth/login.
func (h *PasswordAuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	credentials, ok := readCredentials(w, r)
	if !ok {
		return
	}
	email, err := normalizeEmail(credentials.Email)
	if err != nil || credentials.Password == "" || len(credentials.Password) > maximumPasswordBytes {
		http.Error(w, "invalid email or password", http.StatusBadRequest)
		return
	}

	var userID uuid.UUID
	var storedHash *string
	err = h.pool.QueryRow(r.Context(),
		`SELECT id, password_hash FROM users WHERE lower(email) = $1`, email,
	).Scan(&userID, &storedHash)
	if errors.Is(err, pgx.ErrNoRows) {
		_ = bcrypt.CompareHashAndPassword(dummyPasswordHash, []byte(credentials.Password))
		http.Error(w, "invalid email or password", http.StatusUnauthorized)
		return
	}
	if err != nil {
		http.Error(w, "could not sign in", http.StatusInternalServerError)
		return
	}
	if storedHash == nil || bcrypt.CompareHashAndPassword([]byte(*storedHash), []byte(credentials.Password)) != nil {
		if storedHash == nil {
			_ = bcrypt.CompareHashAndPassword(dummyPasswordHash, []byte(credentials.Password))
		}
		http.Error(w, "invalid email or password", http.StatusUnauthorized)
		return
	}

	SetSessionCookie(w, userID, h.cookieSecret, h.cookieOpts)
	w.WriteHeader(http.StatusNoContent)
}

func readCredentials(w http.ResponseWriter, r *http.Request) (passwordCredentials, bool) {
	if r.Body == nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return passwordCredentials{}, false
	}
	defer r.Body.Close()
	decoder := json.NewDecoder(io.LimitReader(r.Body, maximumAuthBodyBytes))
	decoder.DisallowUnknownFields()
	var credentials passwordCredentials
	if err := decoder.Decode(&credentials); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return passwordCredentials{}, false
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return passwordCredentials{}, false
	}
	return credentials, true
}

func normalizeEmail(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 254 {
		return "", errors.New("invalid email")
	}
	parsed, err := mail.ParseAddress(value)
	if err != nil || !strings.EqualFold(parsed.Address, value) {
		return "", errors.New("invalid email")
	}
	return strings.ToLower(value), nil
}

func validatePassword(password string) error {
	if utf8.RuneCountInString(password) < minimumPasswordRunes || len(password) > maximumPasswordBytes {
		return errors.New("password must be at least 8 characters and no more than 72 bytes")
	}
	return nil
}

func createPasswordUser(ctx context.Context, pool *pgxpool.Pool, email, passwordHash string) (uuid.UUID, error) {
	handle := "u_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:10]
	displayName := strings.SplitN(email, "@", 2)[0]
	var userID uuid.UUID
	err := pool.QueryRow(ctx, `
		INSERT INTO users (handle, display_name, email, password_hash)
		VALUES ($1, $2, $3, $4)
		RETURNING id`, handle, displayName, email, passwordHash,
	).Scan(&userID)
	if err == nil {
		return userID, nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return uuid.Nil, errEmailAlreadyRegistered
	}
	return uuid.Nil, err
}