package main

import (
	"errors"
	"fmt"

	"github.com/AndreiMartynenko/wallet-api/internal/account"
)

func main() {
	acc := account.NewAccount("acc-001", "Alex")
	acc.Deposit(5000) // 5000 pence = £50.00

	err := acc.Withdraw(7000) // try to withdraw £70 - should fail
	if err != nil {
		if errors.Is(err, account.ErrInsufficientFunds) {
			fmt.Println("Blocked: not enough funds -", err)
		} else {
			fmt.Println("Error:", err)
		}
		return
	}
	fmt.Printf("Balance after withdrawal: %d pence\n", acc.Balance)
}
