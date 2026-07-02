package account

import (
	"errors"
	"fmt"
)

// Sentinel errors — reusable, comparable error values.
var (
	ErrInvalidAmount     = errors.New("amount must be positive")
	ErrInsufficientFunds = errors.New("insufficient funds")
)

// Account represents a single wallet account.
type Account struct {
	ID      string
	Owner   string
	Balance int64 // stored in cents to avoid floating point issues with money
}

// NewAccount creates a new account with zero balance.
func NewAccount(id, owner string) *Account {
	return &Account{
		ID:      id,
		Owner:   owner,
		Balance: 0,
	}
}

// Deposit adds money to the account. Returns an error if amount is invalid.
func (a *Account) Deposit(amount int64) error {
	if amount <= 0 {
		return ErrInvalidAmount
	}
	a.Balance += amount
	return nil
}

// Withdraw removes money from the account, if there's enough balance.
func (a *Account) Withdraw(amount int64) error {
	if amount <= 0 {
		return ErrInvalidAmount
	}
	if amount > a.Balance {
		return fmt.Errorf("withdraw %d from balance %d: %w", amount, a.Balance, ErrInsufficientFunds)
	}
	a.Balance -= amount
	return nil
}
