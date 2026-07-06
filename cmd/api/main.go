package main

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/AndreiMartynenko/wallet-api/internal/account"
	"github.com/AndreiMartynenko/wallet-api/internal/api"
)

func main() {
	store := account.NewStore()
	server := api.NewServer(store)

	r := chi.NewRouter()
	r.Post("/accounts", server.CreateAccount)
	r.Get("/accounts/{id}", server.GetAccount)
	r.Post("/accounts/{id}/deposit", server.Deposit)
	r.Post("/accounts/{id}/withdraw", server.Withdraw)

	fmt.Println("Wallet API listening on :8080")
	http.ListenAndServe(":8080", r)
}
