# wallet-api

A small event-driven core banking wallet API, written in Go, backed by
Postgres. Built as a learning project to go from "hello world" to a
JWT-authenticated, idempotent, concurrency-safe money-movement API — see
[wallet-api.md](wallet-api.md) for the day-by-day build log.

## What this demonstrates

- **Double-entry bookkeeping** — every transfer writes a debit and a
  credit row that net to zero, the same invariant real ledgers rely on.
- **Correct concurrency under real load** — `Deposit`, `Withdraw`, and
  `Transfer` all use `SELECT ... FOR UPDATE` inside a SQL transaction, so
  two simultaneous requests against the same account can't produce a lost
  update or an overdraft. Proven with concurrent-goroutine tests run under
  `go test -race`.
- **Idempotent money movement** — `POST /transfer` accepts an
  `Idempotency-Key` header; a retried request replays the original
  response instead of moving money twice.
- **JWT auth with per-resource ownership checks** — every account
  endpoint requires a valid token, and handlers verify the caller actually
  owns the account they're touching (not just that they're logged in).
- **Structured, machine-readable logging** — JSON logs via `log/slog` for
  every request and every state-changing operation (deposit, withdraw,
  transfer), plus consistent `{"error": "..."}` JSON error responses.

## Architecture

```
                 ┌─────────────┐
  HTTP request → │ chi router  │
                 └──────┬──────┘
                        │  LoggingMiddleware (structured request logs)
                        ▼
                 ┌─────────────┐
                 │ auth.       │  verifies JWT, injects username into
                 │ Middleware  │  request context (protected routes only)
                 └──────┬──────┘
                        ▼
                 ┌─────────────┐
                 │  handlers   │  decode request → check ownership →
                 │ (internal/  │  call store → write JSON response
                 │    api)     │
                 └──────┬──────┘
                        ▼
        ┌───────────────┴───────────────┐
        ▼               ▼               ▼
 account.Postgres  user.Postgres  idempotency.
     Store             Store          Store
        │               │               │
        └───────────────┴───────────────┘
                        ▼
                   PostgreSQL
        (accounts, transactions, users, idempotency_keys)
```

Each domain concern lives in its own package under `internal/`, with a
`PostgresStore` that owns its own SQL:

| Package                | Responsibility                                              |
|-------------------------|--------------------------------------------------------------|
| `internal/account`      | Account struct, deposit/withdraw/transfer logic, ledger      |
| `internal/user`         | User records, bcrypt password hashes                        |
| `internal/auth`         | JWT issuing/verification, auth middleware                    |
| `internal/idempotency`  | Idempotency key storage and replay                            |
| `internal/api`          | HTTP handlers, request logging, JSON response helpers        |
| `internal/db`           | Postgres connection pool + SQL migrations                    |

## Getting started

**Prerequisites:** Go 1.26+, Docker, [golang-migrate](https://github.com/golang-migrate/migrate).

```bash
# 1. Start Postgres
docker compose up -d

# 2. Apply migrations
migrate -database "postgres://wallet:wallet@localhost:5433/wallet?sslmode=disable" \
  -path internal/db/migrations up

# 3. Run the API
JWT_SECRET="something-long-and-random" go run ./cmd/api
# Wallet API listening on :8080
```

Run the test suite (needs the same running Postgres instance):

```bash
go test -race ./...
```

## API reference

All request/response bodies are JSON. Protected routes require
`Authorization: Bearer <token>` from `POST /login`.

| Method | Path                          | Auth | Body                                          | Notes                                             |
|--------|-------------------------------|------|------------------------------------------------|----------------------------------------------------|
| POST   | `/register`                   | —    | `{"username","password"}`                     | Creates a user                                     |
| POST   | `/login`                      | —    | `{"username","password"}`                     | Returns `{"token": "..."}`                         |
| POST   | `/accounts`                   | ✓    | `{"id"}`                                       | Owner is always the authenticated user             |
| GET    | `/accounts/{id}`              | ✓    | —                                               | 403 if the account isn't yours                     |
| POST   | `/accounts/{id}/deposit`      | ✓    | `{"amount"}`                                   | Amount in cents                                    |
| POST   | `/accounts/{id}/withdraw`     | ✓    | `{"amount"}`                                   | 402 on insufficient funds                          |
| GET    | `/accounts/{id}/transactions` | ✓    | —                                               | Full transaction history for the account           |
| POST   | `/transfer`                   | ✓    | `{"from_id","to_id","amount"}`                 | Optional `Idempotency-Key` header for safe retries |

Every error response has the shape `{"error": "message"}` with an
appropriate HTTP status code (400/401/402/403/404/409/500).

## Project layout

```
cmd/api/            entrypoint — wires stores, middleware, routes
internal/account/    Account domain type + Postgres-backed store
internal/api/        HTTP handlers, middleware, response helpers
internal/auth/       JWT + bcrypt
internal/user/       user records
internal/idempotency/ idempotency key cache
internal/db/         connection pool + SQL migrations
```
