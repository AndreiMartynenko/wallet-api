package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestHashAndCheckPassword(t *testing.T) {
	hash, err := HashPassword("correct-horse-battery-staple")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}

	if !CheckPassword(hash, "correct-horse-battery-staple") {
		t.Error("CheckPassword: correct password was rejected")
	}
	if CheckPassword(hash, "wrong-password") {
		t.Error("CheckPassword: wrong password was accepted")
	}
}

func TestIssueAndVerifyToken(t *testing.T) {
	issuer := NewTokenIssuer("test-secret")

	token, err := issuer.IssueToken("alex")
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}

	username, err := issuer.VerifyToken(token)
	if err != nil {
		t.Fatalf("VerifyToken: %v", err)
	}
	if username != "alex" {
		t.Errorf("username = %q, want %q", username, "alex")
	}
}

func TestVerifyToken_WrongSecret(t *testing.T) {
	issued := NewTokenIssuer("secret-a")
	verifier := NewTokenIssuer("secret-b")

	token, err := issued.IssueToken("alex")
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}

	if _, err := verifier.VerifyToken(token); err == nil {
		t.Error("VerifyToken: expected error for token signed with a different secret")
	}
}

func TestMiddleware_RejectsMissingAndInvalidTokens(t *testing.T) {
	issuer := NewTokenIssuer("test-secret")
	handlerCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
	})

	cases := []struct {
		name   string
		header string
	}{
		{"no header", ""},
		{"malformed header", "not-a-bearer-token"},
		{"garbage token", "Bearer garbage"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			handlerCalled = false
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}
			w := httptest.NewRecorder()

			issuer.Middleware(next).ServeHTTP(w, req)

			if w.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
			}
			if handlerCalled {
				t.Error("next handler was called despite invalid auth")
			}
		})
	}
}

func TestMiddleware_AllowsValidToken(t *testing.T) {
	issuer := NewTokenIssuer("test-secret")
	token, err := issuer.IssueToken("alex")
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}

	var gotUsername string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUsername = UsernameFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	issuer.Middleware(next).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if gotUsername != "alex" {
		t.Errorf("username in context = %q, want %q", gotUsername, "alex")
	}
}

func TestVerifyToken_Expired(t *testing.T) {
	issuer := NewTokenIssuer("test-secret")

	claims := jwt.RegisteredClaims{
		Subject:   "alex",
		IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
	}
	expired := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := expired.SignedString(issuer.secret)
	if err != nil {
		t.Fatalf("failed to sign expired token: %v", err)
	}

	if _, err := issuer.VerifyToken(tokenString); err == nil {
		t.Error("VerifyToken: expected error for expired token, got nil")
	}
}
