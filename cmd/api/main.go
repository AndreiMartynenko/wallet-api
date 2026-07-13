package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/AndreiMartynenko/wallet-api/internal/db"
	"github.com/go-chi/chi/v5"

	"github.com/AndreiMartynenko/wallet-api/internal/account"
	"github.com/AndreiMartynenko/wallet-api/internal/api"
)

func main() {
	ctx := context.Background()
	pool, err := db.Connect(ctx)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()
	fmt.Println("Connected to Postgres successfully")

	store := account.NewPostgresStore(pool)
	server := api.NewServer(store)

	r := chi.NewRouter()
	r.Post("/accounts", server.CreateAccount)
	r.Get("/accounts/{id}", server.GetAccount)
	r.Post("/accounts/{id}/deposit", server.Deposit)
	r.Post("/accounts/{id}/withdraw", server.Withdraw)
	r.Get("/accounts/{id}/transactions", server.ListTransactions)
	r.Post("/transfer", server.Transfer)

	fmt.Println("Wallet API listening on :8080")
	http.ListenAndServe(":8080", r)
}
