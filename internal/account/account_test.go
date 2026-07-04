package account

import (
	"errors"
	"testing"
)

func errorIs(err, target error) bool {
	return errors.Is(err, target)
}

func TestDeposit(t *testing.T) {
	tests := []struct {
		name          string
		startBalance  int64
		depositAmount int64
		wantBalance   int64
		wantErr       error
	}{
		{"valid deposit", 0, 5000, 5000, nil},
		{"zero amount rejected", 1000, 0, 1000, ErrInvalidAmount},
		{"negative amount rejected", 1000, -500, 1000, ErrInvalidAmount},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			acc := NewAccount("acc-test", "Tester")
			acc.Balance = tt.startBalance

			err := acc.Deposit(tt.depositAmount)

			if tt.wantErr != nil {
				if err != tt.wantErr {
					t.Errorf("got error %v, want %v", err, tt.wantErr)
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if acc.Balance != tt.wantBalance {
				t.Errorf("got balance %d, want %d", acc.Balance, tt.wantBalance)
			}
		})
	}
}

func TestWithdraw(t *testing.T) {
	tests := []struct {
		name           string
		startBalance   int64
		withdrawAmount int64
		wantBalance    int64
		wantErr        error
	}{
		{"valid withdraw", 5000, 2000, 3000, nil},
		{"exact balance withdraw", 5000, 5000, 0, nil},
		{"insufficient funds", 5000, 7000, 5000, ErrInsufficientFunds},
		{"zero amount rejected", 5000, 0, 5000, ErrInvalidAmount},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			acc := NewAccount("acc-test", "Tester")
			acc.Balance = tt.startBalance

			err := acc.Withdraw(tt.withdrawAmount)

			if tt.wantErr != nil {
				if !errorIs(err, tt.wantErr) {
					t.Errorf("got error %v, want %v", err, tt.wantErr)
				}

			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if acc.Balance != tt.wantBalance {
				t.Errorf("got balance %d, want %d", acc.Balance, tt.wantBalance)
			}
		})
	}
}
