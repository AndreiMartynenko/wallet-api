package account

import (
	"context"
	"sync"
	"testing"
)

// TestConcurrentDeposits fires many concurrent deposits at the same account
// and checks the final balance is exactly what it should be. Before the
// Day 16 fix (Get, mutate in Go, then UpdateBalance as separate calls),
// this reliably lost updates under `go test -race`. With row-level
// locking (SELECT ... FOR UPDATE) inside a transaction, every deposit
// is serialized and none get clobbered.
func TestConcurrentDeposits(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	acc := NewAccount("acc-race", "Riley")
	if err := store.Create(ctx, acc); err != nil {
		t.Fatalf("failed to create account: %v", err)
	}

	const goroutines = 50
	const depositAmount = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			if _, err := store.Deposit(ctx, "acc-race", depositAmount); err != nil {
				t.Errorf("deposit failed: %v", err)
			}
		}()
	}
	wg.Wait()

	final, err := store.Get(ctx, "acc-race")
	if err != nil {
		t.Fatalf("failed to fetch final account: %v", err)
	}

	want := int64(goroutines * depositAmount)
	if final.Balance != want {
		t.Errorf("balance = %d, want %d (lost updates under concurrent deposits)", final.Balance, want)
	}
}

// TestConcurrentWithdrawsNeverOverdraw fires concurrent withdrawals that,
// if unsynchronized, could all read the same starting balance and let the
// account go negative. Row locking should serialize them so exactly
// enough withdrawals succeed to drain the account and no more.
func TestConcurrentWithdrawsNeverOverdraw(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	acc := NewAccount("acc-race-2", "Riley")
	if err := store.Create(ctx, acc); err != nil {
		t.Fatalf("failed to create account: %v", err)
	}
	if err := store.UpdateBalance(ctx, "acc-race-2", 1000); err != nil {
		t.Fatalf("failed to seed balance: %v", err)
	}

	const goroutines = 20
	const withdrawAmount = 100 // 10 succeed, 10 fail with insufficient funds

	var wg sync.WaitGroup
	var mu sync.Mutex
	successes := 0

	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			if _, err := store.Withdraw(ctx, "acc-race-2", withdrawAmount); err == nil {
				mu.Lock()
				successes++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if successes != 10 {
		t.Errorf("successful withdrawals = %d, want 10", successes)
	}

	final, err := store.Get(ctx, "acc-race-2")
	if err != nil {
		t.Fatalf("failed to fetch final account: %v", err)
	}
	if final.Balance != 0 {
		t.Errorf("balance = %d, want 0 (overdrawn or under-drawn)", final.Balance)
	}
}
