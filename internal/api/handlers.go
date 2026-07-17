package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/AndreiMartynenko/wallet-api/internal/account"
	"github.com/AndreiMartynenko/wallet-api/internal/auth"
	"github.com/AndreiMartynenko/wallet-api/internal/idempotency"
	"github.com/AndreiMartynenko/wallet-api/internal/user"
)

type Server struct {
	store       *account.PostgresStore
	users       *user.PostgresStore
	tokens      *auth.TokenIssuer
	idempotency *idempotency.Store
}

func NewServer(store *account.PostgresStore, users *user.PostgresStore, tokens *auth.TokenIssuer, idempotencyStore *idempotency.Store) *Server {
	return &Server{store: store, users: users, tokens: tokens, idempotency: idempotencyStore}
}

type createAccountRequest struct {
	ID string `json:"id"`
}

type transferRequest struct {
	FromID string `json:"from_id"`
	ToID   string `json:"to_id"`
	Amount int64  `json:"amount"`
}

// requireOwnership fetches the account and verifies it belongs to the
// authenticated user. It writes the appropriate error response itself
// (404 if the account doesn't exist, 403 if it belongs to someone else)
// and returns ok=false when the caller should stop handling the request.
func (s *Server) requireOwnership(w http.ResponseWriter, r *http.Request, accountID string) (acc *account.Account, ok bool) {
	acc, err := s.store.Get(r.Context(), accountID)
	if errors.Is(err, account.ErrNotFound) {
		writeError(w, http.StatusNotFound, "account not found")
		return nil, false
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch account")
		return nil, false
	}
	if acc.Owner != auth.UsernameFromContext(r.Context()) {
		writeError(w, http.StatusForbidden, "forbidden")
		return nil, false
	}
	return acc, true
}

// CreateAccount creates a new account owned by the authenticated user.
// The owner is always the caller's identity from the JWT — it can't be
// spoofed via the request body.
func (s *Server) CreateAccount(w http.ResponseWriter, r *http.Request) {
	var req createAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	owner := auth.UsernameFromContext(r.Context())
	acc := account.NewAccount(req.ID, owner)
	if err := s.store.Create(r.Context(), acc); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create account")
		return
	}

	slog.Info("account created", "account_id", acc.ID, "owner", acc.Owner)
	writeJSON(w, http.StatusCreated, acc)
}

func (s *Server) GetAccount(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	acc, ok := s.requireOwnership(w, r, id)
	if !ok {
		return
	}

	writeJSON(w, http.StatusOK, acc)
}

type transactionRequest struct {
	Amount int64 `json:"amount"`
}

func (s *Server) Deposit(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if _, ok := s.requireOwnership(w, r, id); !ok {
		return
	}

	var req transactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	acc, err := s.store.Deposit(r.Context(), id, req.Amount)
	if err != nil {
		switch {
		case errors.Is(err, account.ErrNotFound):
			writeError(w, http.StatusNotFound, "account not found")
		case errors.Is(err, account.ErrInvalidAmount):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "failed to save updated balance")
		}
		return
	}

	slog.Info("deposit", "account_id", id, "amount", req.Amount, "new_balance", acc.Balance)
	writeJSON(w, http.StatusOK, acc)
}

func (s *Server) Withdraw(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if _, ok := s.requireOwnership(w, r, id); !ok {
		return
	}

	var req transactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	acc, err := s.store.Withdraw(r.Context(), id, req.Amount)
	if err != nil {
		switch {
		case errors.Is(err, account.ErrNotFound):
			writeError(w, http.StatusNotFound, "account not found")
		case errors.Is(err, account.ErrInsufficientFunds):
			writeError(w, http.StatusPaymentRequired, err.Error())
		case errors.Is(err, account.ErrInvalidAmount):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "failed to save updated balance")
		}
		return
	}

	slog.Info("withdraw", "account_id", id, "amount", req.Amount, "new_balance", acc.Balance)
	writeJSON(w, http.StatusOK, acc)
}

// Transfer moves money between accounts. If the caller sends an
// Idempotency-Key header, a retried request with the same key replays the
// original response instead of re-running the transfer — otherwise a
// dropped response on the client side could cause a network-level retry
// to double-move money.
func (s *Server) Transfer(w http.ResponseWriter, r *http.Request) {
	var req transferRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// You can only move money out of an account you own; the destination
	// can belong to anyone.
	if _, ok := s.requireOwnership(w, r, req.FromID); !ok {
		return
	}

	idempotencyKey := r.Header.Get("Idempotency-Key")
	if idempotencyKey != "" {
		cached, err := s.idempotency.Get(r.Context(), idempotencyKey)
		if err != nil && !errors.Is(err, idempotency.ErrNotFound) {
			writeError(w, http.StatusInternalServerError, "failed to check idempotency key")
			return
		}
		if cached != nil {
			slog.Info("transfer replayed from idempotency key", "idempotency_key", idempotencyKey, "from_id", req.FromID, "to_id", req.ToID)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(cached.Status)
			w.Write(cached.Body)
			return
		}
	}

	status, body := s.doTransfer(r, req)

	if idempotencyKey != "" {
		if err := s.idempotency.Save(r.Context(), idempotencyKey, status, body); err != nil {
			// The transfer itself already succeeded or failed as recorded in
			// status/body — a caching failure shouldn't hide that from the
			// caller, so just log it and still return the real result.
			slog.Error("failed to save idempotency key", "idempotency_key", idempotencyKey, "error", err)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(body)
}

func (s *Server) doTransfer(r *http.Request, req transferRequest) (status int, body []byte) {
	err := s.store.Transfer(r.Context(), req.FromID, req.ToID, req.Amount)
	if err != nil {
		switch {
		case errors.Is(err, account.ErrInsufficientFunds):
			status = http.StatusPaymentRequired
		case errors.Is(err, account.ErrNotFound):
			status = http.StatusNotFound
			err = errors.New("account not found")
		case errors.Is(err, account.ErrInvalidAmount):
			status = http.StatusBadRequest
		default:
			status = http.StatusInternalServerError
			err = errors.New("transfer failed")
		}
		slog.Warn("transfer failed", "from_id", req.FromID, "to_id", req.ToID, "amount", req.Amount, "error", err)
		msg, _ := json.Marshal(errorResponse{Error: err.Error()})
		return status, msg
	}

	slog.Info("transfer complete", "from_id", req.FromID, "to_id", req.ToID, "amount", req.Amount)
	return http.StatusOK, []byte(`{"status":"transfer complete"}`)
}

func (s *Server) ListTransactions(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if _, ok := s.requireOwnership(w, r, id); !ok {
		return
	}

	txs, err := s.store.ListTransactions(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch transactions")
		return
	}

	writeJSON(w, http.StatusOK, txs)
}
