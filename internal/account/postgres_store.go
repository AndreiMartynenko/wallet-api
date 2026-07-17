package account

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("account not found")

type PostgresStore struct {
	pool *pgxpool.Pool
}

type Transaction struct {
	ID              int64     `json:"id"`
	DebitAccountID  string    `json:"debit_account_id"`
	CreditAccountID string    `json:"credit_account_id"`
	Amount          int64     `json:"amount"`
	CreatedAt       time.Time `json:"created_at"`
}

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

func (s *PostgresStore) Create(ctx context.Context, acc *Account) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO accounts (id, owner, balance) VALUES ($1, $2, $3)`,
		acc.ID, acc.Owner, acc.Balance,
	)
	return err
}

func (s *PostgresStore) Get(ctx context.Context, id string) (*Account, error) {
	var acc Account
	err := s.pool.QueryRow(ctx,
		`SELECT id, owner, balance FROM accounts WHERE id = $1`,
		id,
	).Scan(&acc.ID, &acc.Owner, &acc.Balance)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &acc, nil
}

func (s *PostgresStore) UpdateBalance(ctx context.Context, id string, newBalance int64) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE accounts SET balance = $1 WHERE id = $2`,
		newBalance, id,
	)
	return err
}

// Deposit atomically increases an account's balance. It locks the row for
// the duration of the transaction so concurrent deposits/withdraws on the
// same account can't race each other (see the Day 16 in-memory version,
// which read the balance, mutated it in Go, then wrote it back — two
// concurrent requests could both read the same starting balance and one
// update would clobber the other).
func (s *PostgresStore) Deposit(ctx context.Context, id string, amount int64) (*Account, error) {
	if amount <= 0 {
		return nil, ErrInvalidAmount
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var acc Account
	err = tx.QueryRow(ctx,
		`SELECT id, owner, balance FROM accounts WHERE id = $1 FOR UPDATE`,
		id,
	).Scan(&acc.ID, &acc.Owner, &acc.Balance)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	acc.Balance += amount

	if _, err := tx.Exec(ctx,
		`UPDATE accounts SET balance = $1 WHERE id = $2`,
		acc.Balance, id,
	); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &acc, nil
}

// Withdraw atomically decreases an account's balance, holding the same
// row lock as Deposit so the two can never race on the same account.
func (s *PostgresStore) Withdraw(ctx context.Context, id string, amount int64) (*Account, error) {
	if amount <= 0 {
		return nil, ErrInvalidAmount
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var acc Account
	err = tx.QueryRow(ctx,
		`SELECT id, owner, balance FROM accounts WHERE id = $1 FOR UPDATE`,
		id,
	).Scan(&acc.ID, &acc.Owner, &acc.Balance)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	if amount > acc.Balance {
		return nil, ErrInsufficientFunds
	}
	acc.Balance -= amount

	if _, err := tx.Exec(ctx,
		`UPDATE accounts SET balance = $1 WHERE id = $2`,
		acc.Balance, id,
	); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &acc, nil
}

func (s *PostgresStore) Transfer(ctx context.Context, fromID, toID string, amount int64) error {
	if amount <= 0 {
		return ErrInvalidAmount
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) // no-op if already committed

	var fromBalance int64
	err = tx.QueryRow(ctx,
		`SELECT balance FROM accounts WHERE id = $1 FOR UPDATE`,
		fromID,
	).Scan(&fromBalance)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}

	if fromBalance < amount {
		return ErrInsufficientFunds
	}

	_, err = tx.Exec(ctx,
		`UPDATE accounts SET balance = balance - $1 WHERE id = $2`,
		amount, fromID,
	)
	if err != nil {
		return err
	}

	res, err := tx.Exec(ctx,
		`UPDATE accounts SET balance = balance + $1 WHERE id = $2`,
		amount, toID,
	)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return ErrNotFound // toID didn't exist
	}

	_, err = tx.Exec(ctx,
		`INSERT INTO transactions (debit_account_id, credit_account_id, amount) VALUES ($1, $2, $3)`,
		fromID, toID, amount,
	)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (s *PostgresStore) ListTransactions(ctx context.Context, accountID string) ([]Transaction, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, debit_account_id, credit_account_id, amount, created_at
		 FROM transactions
		 WHERE debit_account_id = $1 OR credit_account_id = $1
		 ORDER BY created_at DESC`,
		accountID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var txs []Transaction
	for rows.Next() {
		var t Transaction
		if err := rows.Scan(&t.ID, &t.DebitAccountID, &t.CreditAccountID, &t.Amount, &t.CreatedAt); err != nil {
			return nil, err
		}
		txs = append(txs, t)
	}
	return txs, rows.Err()
}
