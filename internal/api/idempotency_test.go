package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func TestTransfer_IdempotencyKeyReplaysCachedResponse(t *testing.T) {
	_, router := newTestServer(t)
	token := registerAndLogin(t, router, "morgan", "hunter2")

	createAccount(t, router, token, "acc-from")
	createAccount(t, router, token, "acc-to")
	deposit(t, router, token, "acc-from", 10000)

	transferReq := func() *http.Request {
		body := strings.NewReader(`{"from_id":"acc-from","to_id":"acc-to","amount":2500}`)
		req := authedRequest(http.MethodPost, "/transfer", body, token)
		req.Header.Set("Idempotency-Key", "retry-key-1")
		return req
	}

	// First request actually moves the money.
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, transferReq())
	if w1.Code != http.StatusOK {
		t.Fatalf("first transfer: got status %d, want %d, body: %s", w1.Code, http.StatusOK, w1.Body.String())
	}

	// A retry with the same key must NOT move money again — it should
	// just replay the first response.
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, transferReq())
	if w2.Code != http.StatusOK {
		t.Fatalf("retried transfer: got status %d, want %d, body: %s", w2.Code, http.StatusOK, w2.Body.String())
	}
	if w1.Body.String() != w2.Body.String() {
		t.Errorf("retried response body = %q, want identical to first response %q", w2.Body.String(), w1.Body.String())
	}

	fromAcc := getAccount(t, router, token, "acc-from")
	if fromAcc.Balance != 7500 {
		t.Errorf("acc-from balance = %d, want 7500 (retry must not double-debit)", fromAcc.Balance)
	}
	toAcc := getAccount(t, router, token, "acc-to")
	if toAcc.Balance != 2500 {
		t.Errorf("acc-to balance = %d, want 2500 (retry must not double-credit)", toAcc.Balance)
	}
}

func TestTransfer_DifferentIdempotencyKeysAreIndependent(t *testing.T) {
	_, router := newTestServer(t)
	token := registerAndLogin(t, router, "casey", "hunter2")

	createAccount(t, router, token, "acc-from-2")
	createAccount(t, router, token, "acc-to-2")
	deposit(t, router, token, "acc-from-2", 10000)

	for i, key := range []string{"key-a", "key-b"} {
		body := strings.NewReader(`{"from_id":"acc-from-2","to_id":"acc-to-2","amount":1000}`)
		req := authedRequest(http.MethodPost, "/transfer", body, token)
		req.Header.Set("Idempotency-Key", key)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("transfer #%d: got status %d, want %d", i, w.Code, http.StatusOK)
		}
	}

	fromAcc := getAccount(t, router, token, "acc-from-2")
	if fromAcc.Balance != 8000 {
		t.Errorf("acc-from-2 balance = %d, want 8000 (two distinct transfers should both apply)", fromAcc.Balance)
	}
}

func createAccount(t *testing.T, router http.Handler, token, id string) {
	t.Helper()
	body := strings.NewReader(`{"id":"` + id + `"}`)
	req := authedRequest(http.MethodPost, "/accounts", body, token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("createAccount(%q): got status %d, want %d, body: %s", id, w.Code, http.StatusCreated, w.Body.String())
	}
}

func deposit(t *testing.T, router http.Handler, token, id string, amount int64) {
	t.Helper()
	body := strings.NewReader(`{"amount":` + strconv.FormatInt(amount, 10) + `}`)
	req := authedRequest(http.MethodPost, "/accounts/"+id+"/deposit", body, token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("deposit(%q, %d): got status %d, want %d, body: %s", id, amount, w.Code, http.StatusOK, w.Body.String())
	}
}

type accountView struct {
	ID      string
	Owner   string
	Balance int64
}

func getAccount(t *testing.T, router http.Handler, token, id string) accountView {
	t.Helper()
	req := authedRequest(http.MethodGet, "/accounts/"+id, nil, token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("getAccount(%q): got status %d, want %d, body: %s", id, w.Code, http.StatusOK, w.Body.String())
	}
	var acc accountView
	if err := json.Unmarshal(w.Body.Bytes(), &acc); err != nil {
		t.Fatalf("getAccount(%q): failed to decode response: %v", id, err)
	}
	return acc
}
