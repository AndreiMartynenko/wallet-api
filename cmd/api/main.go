package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"

	"github.com/AndreiMartynenko/wallet-api/internal/db"
	"github.com/go-chi/chi/v5"

	"github.com/AndreiMartynenko/wallet-api/internal/account"
	"github.com/AndreiMartynenko/wallet-api/internal/api"
	"github.com/AndreiMartynenko/wallet-api/internal/auth"
	"github.com/AndreiMartynenko/wallet-api/internal/idempotency"
	"github.com/AndreiMartynenko/wallet-api/internal/user"
)

func jwtSecret() string {
	if s := os.Getenv("JWT_SECRET"); s != "" {
		return s
	}
	log.Println("warning: JWT_SECRET not set, using an insecure dev default — do not use in production")
	return "dev-only-insecure-secret"
}

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	ctx := context.Background()
	pool, err := db.Connect(ctx)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()
	fmt.Println("Connected to Postgres successfully")

	store := account.NewPostgresStore(pool)
	users := user.NewPostgresStore(pool)
	tokens := auth.NewTokenIssuer(jwtSecret())
	idempotencyStore := idempotency.NewStore(pool)
	server := api.NewServer(store, users, tokens, idempotencyStore)

	r := chi.NewRouter()
	r.Use(api.LoggingMiddleware)

	// Public routes — no account data, so no auth required.
	r.Post("/register", server.Register)
	r.Post("/login", server.Login)

	// Everything below touches account data and requires a valid JWT;
	// handlers additionally check that the account belongs to the caller.
	r.Group(func(r chi.Router) {
		r.Use(tokens.Middleware)
		r.Post("/accounts", server.CreateAccount)
		r.Get("/accounts/{id}", server.GetAccount)
		r.Post("/accounts/{id}/deposit", server.Deposit)
		r.Post("/accounts/{id}/withdraw", server.Withdraw)
		r.Get("/accounts/{id}/transactions", server.ListTransactions)
		r.Post("/transfer", server.Transfer)
	})

	fmt.Println("Wallet API listening on :8080")
	http.ListenAndServe(":8080", r)
}
