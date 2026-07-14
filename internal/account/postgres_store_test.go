package account

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func setupTestStore(t *testing.T) *PostgresStore {
	t.Helper()

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, "postgres://wallet:wallet@localhost:5433/wallet")
	if err != nil {
		t.Fatalf("failed to connect to test database: %v", err)
	}

	t.Cleanup(func() {
		pool.Close()
	})

	// Clean slate before each test.
	_, err = pool.Exec(ctx, "DELETE FROM transactions")
	if err != nil {
		t.Fatalf("failed to clean transactions table: %v", err)
	}
	_, err = pool.Exec(ctx, "DELETE FROM accounts")
	if err != nil {
		t.Fatalf("failed to clean accounts table: %v", err)
	}

	return NewPostgresStore(pool)
}

func TestTransfer_Success(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	accA := NewAccount("acc-a", "Alex")
	accB := NewAccount("acc-b", "Sam")
	store.Create(ctx, accA)
	store.Create(ctx, accB)
	store.UpdateBalance(ctx, "acc-a", 10000)

	err := store.Transfer(ctx, "acc-a", "acc-b", 4000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	updatedA, _ := store.Get(ctx, "acc-a")
	updatedB, _ := store.Get(ctx, "acc-b")

	if updatedA.Balance != 6000 {
		t.Errorf("acc-a balance = %d, want 6000", updatedA.Balance)
	}
	if updatedB.Balance != 4000 {
		t.Errorf("acc-b balance = %d, want 4000", updatedB.Balance)
	}
}

func TestTransfer_InsufficientFunds(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	accA := NewAccount("acc-a", "Alex")
	accB := NewAccount("acc-b", "Sam")
	store.Create(ctx, accA)
	store.Create(ctx, accB)
	store.UpdateBalance(ctx, "acc-a", 1000)

	err := store.Transfer(ctx, "acc-a", "acc-b", 5000)
	if err != ErrInsufficientFunds {
		t.Fatalf("got error %v, want ErrInsufficientFunds", err)
	}

	// Balances must be unchanged — the failed transfer should not
	// have modified anything (this proves the rollback worked).
	updatedA, _ := store.Get(ctx, "acc-a")
	if updatedA.Balance != 1000 {
		t.Errorf("acc-a balance = %d, want unchanged 1000", updatedA.Balance)
	}
}

func TestTransfer_NonexistentDestination(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	accA := NewAccount("acc-a", "Alex")
	store.Create(ctx, accA)
	store.UpdateBalance(ctx, "acc-a", 5000)

	err := store.Transfer(ctx, "acc-a", "does-not-exist", 1000)
	if err != ErrNotFound {
		t.Fatalf("got error %v, want ErrNotFound", err)
	}

	// Balance must be unchanged — rollback should have undone the debit.
	updatedA, _ := store.Get(ctx, "acc-a")
	if updatedA.Balance != 5000 {
		t.Errorf("acc-a balance = %d, want unchanged 5000 (rollback failed!)", updatedA.Balance)
	}
}
