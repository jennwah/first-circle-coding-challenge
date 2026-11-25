package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/shopspring/decimal"

	"github.com/leejennwah/first-circle-challenge/models"
	"github.com/leejennwah/first-circle-challenge/repository"
	"github.com/leejennwah/first-circle-challenge/service"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://postgres:postgres@localhost:5432/bank?sslmode=disable"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	repo, err := repository.Open(ctx, dsn)
	if err != nil {
		return err
	}
	defer repo.Close()

	var bankService service.Bank = service.New(repo)

	alice, err := bankService.CreateAccount(ctx, "Alice", decimal.NewFromInt(100))
	if err != nil {
		return err
	}
	bob, err := bankService.CreateAccount(ctx, "Bob", decimal.NewFromInt(50))
	if err != nil {
		return err
	}
	fmt.Printf("created %s (#%d) with %s\n", alice.Owner, alice.ID, alice.Balance)
	fmt.Printf("created %s (#%d) with %s\n", bob.Owner, bob.ID, bob.Balance)

	deposit, err := bankService.Deposit(ctx, alice.ID, decimal.RequireFromString("25.50"))
	if err != nil {
		return err
	}
	fmt.Printf("deposited %s to %s, balance %s\n", deposit.Amount, alice.Owner, deposit.BalanceAfter)

	withdrawal, err := bankService.Withdraw(ctx, bob.ID, decimal.RequireFromString("20.00"))
	if err != nil {
		return err
	}
	fmt.Printf("withdrew %s from %s, balance %s\n", withdrawal.Amount, bob.Owner, withdrawal.BalanceAfter)

	transfer, err := bankService.Transfer(ctx, alice.ID, bob.ID, decimal.RequireFromString("75.25"))
	if err != nil {
		return err
	}
	fmt.Printf("transferred %s from %s to %s (transfer %s)\n", transfer.From.Amount, alice.Owner, bob.Owner, transfer.ID)
	fmt.Printf("balances after transfer: %s %s, %s %s\n", alice.Owner, transfer.From.BalanceAfter, bob.Owner, transfer.To.BalanceAfter)

	if _, err := bankService.Withdraw(ctx, bob.ID, decimal.RequireFromString("1000.00")); errors.Is(err, models.ErrInsufficientFunds) {
		fmt.Printf("overdraft on %s rejected as expected\n", bob.Owner)
	} else if err != nil {
		return err
	}

	for _, account := range []*models.Account{alice, bob} {
		balance, err := bankService.Balance(ctx, account.ID)
		if err != nil {
			return err
		}
		fmt.Printf("%s final balance %s\n", account.Owner, balance)
	}

	entries, err := bankService.Transactions(ctx, alice.ID)
	if err != nil {
		return err
	}
	fmt.Printf("%s ledger:\n", alice.Owner)
	for _, entry := range entries {
		fmt.Printf("  %-13s %10s -> %s\n", entry.Type, entry.Amount, entry.BalanceAfter)
	}

	return nil
}
