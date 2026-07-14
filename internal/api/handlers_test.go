package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/AndreiMartynenko/wallet-api/internal/account"
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

	store := account.NewPostgresStore(pool)
	server := NewServer(store)

	r := chi.NewRouter()
	r.Post("/accounts", server.CreateAccount)
	r.Get("/accounts/{id}", server.GetAccount)
	r.Post("/accounts/{id}/deposit", server.Deposit)
	r.Post("/accounts/{id}/withdraw", server.Withdraw)

	return server, r
}

func TestCreateAccount(t *testing.T) {
	_, router := newTestServer(t)

	body := strings.NewReader(`{"id":"acc-1","owner":"Alex"}`)
	req := httptest.NewRequest(http.MethodPost, "/accounts", body)
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

	// Create the account first.
	createBody := strings.NewReader(`{"id":"acc-2","owner":"Sam"}`)
	createReq := httptest.NewRequest(http.MethodPost, "/accounts", createBody)
	createW := httptest.NewRecorder()
	router.ServeHTTP(createW, createReq)

	// Deposit.
	depBody := strings.NewReader(`{"amount":5000}`)
	depReq := httptest.NewRequest(http.MethodPost, "/accounts/acc-2/deposit", depBody)
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
	wdReq := httptest.NewRequest(http.MethodPost, "/accounts/acc-2/withdraw", wdBody)
	wdW := httptest.NewRecorder()
	router.ServeHTTP(wdW, wdReq)

	if wdW.Code != http.StatusPaymentRequired {
		t.Errorf("withdraw: got status %d, want %d", wdW.Code, http.StatusPaymentRequired)
	}
}

func TestGetAccountNotFound(t *testing.T) {
	_, router := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/accounts/does-not-exist", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("got status %d, want %d", w.Code, http.StatusNotFound)
	}
}
