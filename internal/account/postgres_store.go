package account

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("account not found")

type PostgresStore struct {
	pool *pgxpool.Pool
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
