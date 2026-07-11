package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/AndreiMartynenko/wallet-api/internal/account"
)

type Server struct {
	store *account.PostgresStore
}

func NewServer(store *account.PostgresStore) *Server {
	return &Server{store: store}
}

type createAccountRequest struct {
	ID    string `json:"id"`
	Owner string `json:"owner"`
}

type transferRequest struct {
	FromID string `json:"from_id"`
	ToID   string `json:"to_id"`
	Amount int64  `json:"amount"`
}

func (s *Server) CreateAccount(w http.ResponseWriter, r *http.Request) {
	var req createAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	acc := account.NewAccount(req.ID, req.Owner)
	if err := s.store.Create(r.Context(), acc); err != nil {
		http.Error(w, "failed to create account", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(acc)
}

func (s *Server) GetAccount(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	acc, err := s.store.Get(r.Context(), id)
	if errors.Is(err, account.ErrNotFound) {
		http.Error(w, "account not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "failed to fetch account", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(acc)
}

type transactionRequest struct {
	Amount int64 `json:"amount"`
}

func (s *Server) Deposit(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	acc, err := s.store.Get(r.Context(), id)
	if errors.Is(err, account.ErrNotFound) {
		http.Error(w, "account not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "failed to fetch account", http.StatusInternalServerError)
		return
	}

	var req transactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if err := acc.Deposit(req.Amount); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := s.store.UpdateBalance(r.Context(), acc.ID, acc.Balance); err != nil {
		http.Error(w, "failed to save updated balance", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(acc)
}

func (s *Server) Withdraw(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	acc, err := s.store.Get(r.Context(), id)
	if errors.Is(err, account.ErrNotFound) {
		http.Error(w, "account not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "failed to fetch account", http.StatusInternalServerError)
		return
	}

	var req transactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if err := acc.Withdraw(req.Amount); err != nil {
		if errors.Is(err, account.ErrInsufficientFunds) {
			http.Error(w, err.Error(), http.StatusPaymentRequired) // 402
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := s.store.UpdateBalance(r.Context(), acc.ID, acc.Balance); err != nil {
		http.Error(w, "failed to save updated balance", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(acc)
}

func (s *Server) Transfer(w http.ResponseWriter, r *http.Request) {
	var req transferRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	err := s.store.Transfer(r.Context(), req.FromID, req.ToID, req.Amount)
	if err != nil {
		switch {
		case errors.Is(err, account.ErrInsufficientFunds):
			http.Error(w, err.Error(), http.StatusPaymentRequired)
		case errors.Is(err, account.ErrNotFound):
			http.Error(w, "account not found", http.StatusNotFound)
		case errors.Is(err, account.ErrInvalidAmount):
			http.Error(w, err.Error(), http.StatusBadRequest)
		default:
			http.Error(w, "transfer failed", http.StatusInternalServerError)
		}
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"transfer complete"}`))
}
