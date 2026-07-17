package idempotency

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("idempotency key not found")

// Response is a previously-recorded HTTP response, cached so a retried
// request with the same Idempotency-Key can be replayed verbatim instead
// of being re-executed (which, for money movement, could double-charge).
type Response struct {
	Status int
	Body   []byte
}

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) Get(ctx context.Context, key string) (*Response, error) {
	var resp Response
	err := s.pool.QueryRow(ctx,
		`SELECT response_status, response_body FROM idempotency_keys WHERE key = $1`,
		key,
	).Scan(&resp.Status, &resp.Body)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// Save records the response for key. If another request already saved a
// response for this key in the meantime (a race between two concurrent
// retries), the conflict is ignored — the first writer's response wins,
// which is what both callers should end up replaying anyway.
func (s *Store) Save(ctx context.Context, key string, status int, body []byte) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO idempotency_keys (key, response_status, response_body)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (key) DO NOTHING`,
		key, status, body,
	)
	return err
}
