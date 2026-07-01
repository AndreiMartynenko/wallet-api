# Wallet API Build Plan — Learn Go by Shipping

**Goal:** Ship a working, portfolio-ready Event-Driven Core Banking Wallet API in ~4 weeks, learning each Go concept the same day you apply it.

**Daily rhythm:**
- 30 min — Exercism puzzle (warm-up)
- 2–2.5 hrs — Today's concept + feature (below)
- 30 min — Read one doc/article on the day's topic (deepen understanding)
- Film 1–2 videos: show the code, explain the concept simply, say what confused you

Commit to GitHub every single day, even tiny progress. The commit history itself becomes proof of consistency to recruiters.

---

## Week 1 — Foundations + Basic API

**Day 1 — Project setup + Go module basics**
- Concept: `go mod init`, project structure, packages, imports
- Task: Initialize repo, create folder structure (`/cmd`, `/internal`, `/pkg`), write a "Hello Wallet" main.go
- Resource: Go Tour (Packages/Basics section)

**Day 2 — Structs, methods, interfaces**
- Concept: How Go does "OOP" without classes — structs + methods + interfaces
- Task: Define `Account` struct (ID, Owner, Balance, Currency), write methods like `(a *Account) Deposit(amount)`
- Resource: Go by Example — Structs, Methods, Interfaces

**Day 3 — Error handling (idiomatic Go)**
- Concept: Go's explicit error returns, custom error types, `errors.Is`/`errors.As`
- Task: Add validation errors (e.g., negative deposit, insufficient funds) to Account methods
- Resource: Go by Example — Errors

**Day 4 — HTTP server basics**
- Concept: `net/http`, routing, handlers, JSON encoding/decoding
- Task: Stand up a basic server with `POST /accounts` (create account) and `GET /accounts/{id}` (fetch balance) — in-memory storage for now
- Resource: Go by Example — HTTP Server

**Day 5 — Router + middleware pattern**
- Concept: Using a lightweight router (Chi or Gin), middleware chain pattern
- Task: Swap raw `net/http` for Chi, add a logging middleware
- Resource: Chi router README/examples

**Day 6 — Testing basics (table-driven tests)**
- Concept: Go's `testing` package, table-driven test pattern, `go test`
- Task: Write tests for Account methods (deposit, withdraw, insufficient funds cases)
- Resource: Learn Go with Tests (first few chapters)

**Day 7 — Rest + review**
- Task: Review the week, fix any broken tests, clean up code, write a "Week 1 recap" video (this is a strong content piece — show the whole week's progress in one video)

---

## Week 2 — Database + Real Persistence

**Day 8 — PostgreSQL + database/sql basics**
- Concept: Connecting Go to Postgres, `database/sql`, connection pooling
- Task: Docker-compose a Postgres instance, connect from Go, run a raw query
- Resource: Go by Example — database/sql (or use `sqlx` for convenience)

**Day 9 — Migrations + schema design**
- Concept: Database migrations (use `golang-migrate`), schema design for accounts + transactions tables
- Task: Write migration for `accounts` table and `transactions` table (double-entry: every transaction row has `debit_account_id`, `credit_account_id`, `amount`)
- Resource: golang-migrate docs

**Day 10 — Replace in-memory storage with Postgres**
- Concept: Repository pattern (separating storage logic from business logic)
- Task: Implement `AccountRepository` interface backed by Postgres; wire it into handlers
- Resource: Read about Repository pattern in Go (Go by Example doesn't cover this — search "Go repository pattern")

**Day 11 — Database transactions (ACID)**
- Concept: SQL transactions in Go (`BEGIN`/`COMMIT`/`ROLLBACK`), why they matter for money movement
- Task: Implement `Transfer(fromID, toID, amount)` — must be atomic (both debit and credit succeed, or neither does)
- Resource: Learn Go with Tests has no DB chapter — read PostgreSQL docs on transactions + Go's `sql.Tx`

**Day 12 — Double-entry bookkeeping logic**
- Concept: Why every transaction needs a debit AND credit row that nets to zero (this is your finance background — teach this one confidently)
- Task: Refactor Transfer to write proper double-entry ledger rows, add a function to verify ledger integrity (sum of all entries = 0)
- Content gold: this is one of your best possible videos — "the accounting trick that keeps banks honest"

**Day 13 — Testing with a real database**
- Concept: Integration tests vs unit tests, test database setup/teardown
- Task: Write integration tests for the Transfer function (including a failure case — insufficient funds should roll back cleanly)
- Resource: Learn Go with Tests — check if it has a DB testing chapter; otherwise search "Go integration testing postgres"

**Day 14 — Rest + review**
- Task: Week 2 recap video, push all code, clean up

---

## Week 3 — Concurrency, Auth, Idempotency

**Day 15 — Goroutines and channels (the concept)**
- Concept: What goroutines are, how channels work, `sync.WaitGroup`
- Task: Small standalone exercise — NOT on the wallet yet — write a tiny program that processes a list of "transactions" concurrently and reports results via a channel
- Resource: Go Tour — Concurrency section, Go by Example — Goroutines/Channels

**Day 16 — Race conditions (the real problem)**
- Concept: What happens when two goroutines write to the same data — race conditions, `go test -race`
- Task: Deliberately write a buggy version where two simultaneous transfers corrupt a balance. Run `-race` and see it flagged. This is a great "aha" video.
- Resource: Go blog — "Introducing the Go Race Detector"

**Day 17 — Fixing concurrency: row-level locking**
- Concept: `SELECT ... FOR UPDATE` in Postgres, how it prevents the race condition from Day 16
- Task: Fix your Transfer function using row locking inside the DB transaction. Re-run the race test — should now be safe.
- Content gold: "What happens if two people spend the same $10 at the same time? (and how I fixed it)"

**Day 18 — JWT authentication**
- Concept: JWT structure, signing/verifying tokens, auth middleware
- Task: Add `POST /login`, protect account endpoints so users can only see/transfer their own accounts
- Resource: `golang-jwt/jwt` package docs

**Day 19 — Idempotency keys**
- Concept: Why retried API requests are dangerous for money movement, idempotency key pattern
- Task: Add `Idempotency-Key` header support to the Transfer endpoint — store keys, return cached result if request is retried
- Content gold: "How I stopped my API from accidentally charging someone twice"

**Day 20 — Structured logging + error responses**
- Concept: Consistent JSON error responses, structured logging (`slog` — Go's built-in structured logger)
- Task: Clean up all error paths to return consistent JSON error shapes; add structured logs for every transaction

**Day 21 — Rest + review**
- Task: Week 3 recap, this is your most "senior-sounding" week — lean into it for content (concurrency + auth + idempotency all in one week is genuinely strong)

---

## Week 4 — Polish, Deploy, Ship

**Day 22 — Docker + docker-compose**
- Concept: Containerizing a Go app, multi-stage builds
- Task: Write a Dockerfile for the API, docker-compose for API + Postgres together

**Day 23 — Deployment**
- Concept: Deploying to Fly.io or Render (free tier)
- Task: Get the API live on a public URL

**Day 24 — API documentation**
- Concept: OpenAPI/Swagger basics
- Task: Add Swagger docs (use `swaggo/swag` for Go) so the API is self-documenting — recruiters can literally click through it

**Day 25 — Stretch feature: rate limiting**
- Concept: Token bucket / sliding window rate limiting
- Task: Add basic rate limiting middleware to protect endpoints

**Day 26 — Stretch feature: simple fraud rule**
- Concept: Basic rule-based flagging (your finance background shines here)
- Task: Flag transactions over a threshold or unusual frequency, log/store as "flagged" — doesn't need to be ML, just rule-based

**Day 27 — README + polish**
- Task: Write a genuinely good README (architecture diagram, how to run it, what it demonstrates) — this is often the first thing a recruiter actually reads

**Day 28 — Final review + "the whole build" video**
- Task: Record a longer-form video (5-10 min) walking through the entire project end-to-end — this becomes your best single piece of content to link in job applications and LinkedIn

---

## Content mapping cheat-sheet

Use this to know what "lens" (from earlier plan) fits each day:

| Day type | Lens to use |
|---|---|
| New concept intro | Concept explainer |
| Hit a bug/confusion | Struggle |
| Feature working | Progress / Milestone |
| Week recap | Reflection |
| Race condition, idempotency, double-entry days | These are your **best hook days** — always reframe as relatable problems ("what if two people spend the same $10 at once") |

## Tracking (update weekly)
- Days coded:
- GitHub commits:
- Videos posted:
- Job applications sent:
- Interviews/responses:
