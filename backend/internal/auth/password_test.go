package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestNormalizeEmail(t *testing.T) {
	got, err := normalizeEmail("  Person.Example@Example.COM  ")
	if err != nil {
		t.Fatalf("normalizeEmail returned error: %v", err)
	}
	if got != "person.example@example.com" {
		t.Fatalf("normalizeEmail = %q", got)
	}

	for _, input := range []string{"", "not-an-email", "Name <person@example.com>"} {
		if _, err := normalizeEmail(input); err == nil {
			t.Errorf("normalizeEmail(%q) expected an error", input)
		}
	}
}

func TestValidatePassword(t *testing.T) {
	if err := validatePassword("correct horse"); err != nil {
		t.Fatalf("valid password rejected: %v", err)
	}
	if err := validatePassword("short"); err == nil {
		t.Fatal("short password accepted")
	}
	if err := validatePassword(strings.Repeat("a", maximumPasswordBytes+1)); err == nil {
		t.Fatal("password beyond bcrypt byte limit accepted")
	}
}

func TestPasswordHashIsVerifiableAndNotPlaintext(t *testing.T) {
	password := "a secure example password"
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("GenerateFromPassword: %v", err)
	}
	if string(hash) == password {
		t.Fatal("stored value is plaintext")
	}
	if err := bcrypt.CompareHashAndPassword(hash, []byte(password)); err != nil {
		t.Fatalf("correct password did not verify: %v", err)
	}
	if err := bcrypt.CompareHashAndPassword(hash, []byte("wrong password")); err == nil {
		t.Fatal("incorrect password verified")
	}
}

func TestReadCredentialsRejectsMalformedOrUnknownFields(t *testing.T) {
	for _, body := range []string{
		`{`,
		`{"email":"a@example.com","password":"long-enough","handle":"alice"}`,
		`{"email":"a@example.com","password":"long-enough"} {}`,
	} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(body))
		if _, ok := readCredentials(recorder, request); ok {
			t.Errorf("readCredentials accepted %q", body)
		}
		if recorder.Code != http.StatusBadRequest {
			t.Errorf("status for %q = %d, want 400", body, recorder.Code)
		}
	}
}