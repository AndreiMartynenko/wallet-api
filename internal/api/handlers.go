package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/AndreiMartynenko/wallet-api/internal/account"
)

type Server struct {
	store *account.Store
}

func NewServer(store *account.Store) *Server {
	return &Server{store: store}
}

type createAccountRequest struct {
	ID    string `json:"id"`
	Owner string `json:"owner"`
}

func (s *Server) CreateAccount(w http.ResponseWriter, r *http.Request) {
	var req createAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	acc := account.NewAccount(req.ID, req.Owner)
	s.store.Create(acc)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(acc)
}

func (s *Server) GetAccount(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	acc, ok := s.store.Get(id)
	if !ok {
		http.Error(w, "account not found", http.StatusNotFound)
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

	acc, ok := s.store.Get(id)
	if !ok {
		http.Error(w, "account not found", http.StatusNotFound)
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(acc)
}

func (s *Server) Withdraw(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	acc, ok := s.store.Get(id)
	if !ok {
		http.Error(w, "account not found", http.StatusNotFound)
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(acc)
}
