package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/AndreiMartynenko/wallet-api/internal/account"
	"github.com/AndreiMartynenko/wallet-api/internal/auth"
	"github.com/AndreiMartynenko/wallet-api/internal/idempotency"
	"github.com/AndreiMartynenko/wallet-api/internal/user"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func newTestServer(t *testing.T) (*Server, *chi.Mux) {
	t.Helper()

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, "postgres://wallet:wallet@localhost:5433/wallet")
	if err != nil {
		t.Fatalf("failed to connect to test database: %v", err)
	}
	t.Cleanup(func() { pool.Close() })

	pool.Exec(ctx, "DELETE FROM transactions")
	pool.Exec(ctx, "DELETE FROM accounts")
	pool.Exec(ctx, "DELETE FROM users")
	pool.Exec(ctx, "DELETE FROM idempotency_keys")

	store := account.NewPostgresStore(pool)
	users := user.NewPostgresStore(pool)
	tokens := auth.NewTokenIssuer("test-secret")
	idempotencyStore := idempotency.NewStore(pool)
	server := NewServer(store, users, tokens, idempotencyStore)

	r := chi.NewRouter()
	r.Post("/register", server.Register)
	r.Post("/login", server.Login)
	r.Group(func(r chi.Router) {
		r.Use(tokens.Middleware)
		r.Post("/accounts", server.CreateAccount)
		r.Get("/accounts/{id}", server.GetAccount)
		r.Post("/accounts/{id}/deposit", server.Deposit)
		r.Post("/accounts/{id}/withdraw", server.Withdraw)
		r.Get("/accounts/{id}/transactions", server.ListTransactions)
		r.Post("/transfer", server.Transfer)
	})

	return server, r
}

// registerAndLogin creates a user and returns a valid bearer token for them.
func registerAndLogin(t *testing.T, router *chi.Mux, username, password string) string {
	t.Helper()

	regBody := strings.NewReader(`{"username":"` + username + `","password":"` + password + `"}`)
	regReq := httptest.NewRequest(http.MethodPost, "/register", regBody)
	regW := httptest.NewRecorder()
	router.ServeHTTP(regW, regReq)
	if regW.Code != http.StatusCreated {
		t.Fatalf("register: got status %d, want %d, body: %s", regW.Code, http.StatusCreated, regW.Body.String())
	}

	loginBody := strings.NewReader(`{"username":"` + username + `","password":"` + password + `"}`)
	loginReq := httptest.NewRequest(http.MethodPost, "/login", loginBody)
	loginW := httptest.NewRecorder()
	router.ServeHTTP(loginW, loginReq)
	if loginW.Code != http.StatusOK {
		t.Fatalf("login: got status %d, want %d, body: %s", loginW.Code, http.StatusOK, loginW.Body.String())
	}

	var resp struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(loginW.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode login response: %v", err)
	}
	return resp.Token
}

func authedRequest(method, target string, body *strings.Reader, token string) *http.Request {
	var req *http.Request
	if body == nil {
		req = httptest.NewRequest(method, target, nil)
	} else {
		req = httptest.NewRequest(method, target, body)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	return req
}

func TestCreateAccount(t *testing.T) {
	_, router := newTestServer(t)
	token := registerAndLogin(t, router, "alex", "hunter2")

	body := strings.NewReader(`{"id":"acc-1"}`)
	req := authedRequest(http.MethodPost, "/accounts", body, token)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("got status %d, want %d", w.Code, http.StatusCreated)
	}

	if !strings.Contains(w.Body.String(), `"acc-1"`) {
		t.Errorf("response body missing account id: %s", w.Body.String())
	}
}

func TestDepositAndWithdrawEndpoints(t *testing.T) {
	_, router := newTestServer(t)
	token := registerAndLogin(t, router, "sam", "hunter2")

	// Create the account first.
	createBody := strings.NewReader(`{"id":"acc-2"}`)
	createReq := authedRequest(http.MethodPost, "/accounts", createBody, token)
	createW := httptest.NewRecorder()
	router.ServeHTTP(createW, createReq)

	// Deposit.
	depBody := strings.NewReader(`{"amount":5000}`)
	depReq := authedRequest(http.MethodPost, "/accounts/acc-2/deposit", depBody, token)
	depW := httptest.NewRecorder()
	router.ServeHTTP(depW, depReq)

	if depW.Code != http.StatusOK {
		t.Fatalf("deposit: got status %d, want %d, body: %s", depW.Code, http.StatusOK, depW.Body.String())
	}
	if !strings.Contains(depW.Body.String(), `"Balance":5000`) {
		t.Errorf("deposit: unexpected body: %s", depW.Body.String())
	}

	// Overdraw attempt — should fail with 402.
	wdBody := strings.NewReader(`{"amount":7000}`)
	wdReq := authedRequest(http.MethodPost, "/accounts/acc-2/withdraw", wdBody, token)
	wdW := httptest.NewRecorder()
	router.ServeHTTP(wdW, wdReq)

	if wdW.Code != http.StatusPaymentRequired {
		t.Errorf("withdraw: got status %d, want %d", wdW.Code, http.StatusPaymentRequired)
	}
}

func TestGetAccountNotFound(t *testing.T) {
	_, router := newTestServer(t)
	token := registerAndLogin(t, router, "riley", "hunter2")

	req := authedRequest(http.MethodGet, "/accounts/does-not-exist", nil, token)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("got status %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestUnauthenticatedRequestsAreRejected(t *testing.T) {
	_, router := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/accounts/acc-1", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("got status %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestCannotAccessAnotherUsersAccount(t *testing.T) {
	_, router := newTestServer(t)
	aliceToken := registerAndLogin(t, router, "alice", "hunter2")
	bobToken := registerAndLogin(t, router, "bob", "hunter2")

	createBody := strings.NewReader(`{"id":"acc-alice"}`)
	createReq := authedRequest(http.MethodPost, "/accounts", createBody, aliceToken)
	createW := httptest.NewRecorder()
	router.ServeHTTP(createW, createReq)
	if createW.Code != http.StatusCreated {
		t.Fatalf("create account: got status %d, want %d", createW.Code, http.StatusCreated)
	}

	// Bob tries to read Alice's account.
	getReq := authedRequest(http.MethodGet, "/accounts/acc-alice", nil, bobToken)
	getW := httptest.NewRecorder()
	router.ServeHTTP(getW, getReq)
	if getW.Code != http.StatusForbidden {
		t.Errorf("bob reading alice's account: got status %d, want %d", getW.Code, http.StatusForbidden)
	}

	// Bob tries to withdraw from Alice's account.
	wdBody := strings.NewReader(`{"amount":100}`)
	wdReq := authedRequest(http.MethodPost, "/accounts/acc-alice/withdraw", wdBody, bobToken)
	wdW := httptest.NewRecorder()
	router.ServeHTTP(wdW, wdReq)
	if wdW.Code != http.StatusForbidden {
		t.Errorf("bob withdrawing from alice's account: got status %d, want %d", wdW.Code, http.StatusForbidden)
	}
}
