package integrationtests

import (
	"context"
	"errors"
	"os"
	"sync"
	"sync/atomic"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"

	"github.com/leejennwah/first-circle-challenge/models"
	"github.com/leejennwah/first-circle-challenge/repository"
	"github.com/leejennwah/first-circle-challenge/service"
)

func newBank(t *testing.T) (service.Bank, context.Context) {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://postgres:postgres@localhost:5432/bank_test?sslmode=disable"
	}

	ctx := context.Background()
	repo, err := repository.Open(ctx, dsn)
	if err != nil {
		t.Skipf("postgres unavailable: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	admin, err := sqlx.ConnectContext(ctx, "pgx", dsn)
	if err != nil {
		t.Fatalf("connect test database: %v", err)
	}
	t.Cleanup(func() { _ = admin.Close() })

	if _, err := admin.ExecContext(ctx, `TRUNCATE transactions, accounts RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("reset database: %v", err)
	}

	var bank service.Bank = service.New(repo)
	return bank, ctx
}

func TestCreateAccountRecordsInitialDeposit(t *testing.T) {
	bank, ctx := newBank(t)

	account, err := bank.CreateAccount(ctx, "Alice", decimal.NewFromInt(100))
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	if account.Owner != "Alice" {
		t.Fatalf("owner = %q, want Alice", account.Owner)
	}
	if !account.Balance.Equal(decimal.NewFromInt(100)) {
		t.Fatalf("balance = %s, want 100", account.Balance)
	}

	entries, err := bank.Transactions(ctx, account.ID)
	if err != nil {
		t.Fatalf("transactions: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(entries))
	}
	if entries[0].Type != models.TypeDeposit {
		t.Fatalf("type = %s, want %s", entries[0].Type, models.TypeDeposit)
	}
}

func TestCreateAccountWithZeroInitialDepositHasNoLedgerEntries(t *testing.T) {
	bank, ctx := newBank(t)

	account, err := bank.CreateAccount(ctx, "Alice", decimal.Zero)
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	entries, err := bank.Transactions(ctx, account.ID)
	if err != nil {
		t.Fatalf("transactions: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("entries = %d, want 0", len(entries))
	}
}

func TestCreateAccountValidation(t *testing.T) {
	bank, ctx := newBank(t)

	cases := []struct {
		name    string
		owner   string
		initial decimal.Decimal
		want    error
	}{
		{name: "blank owner", owner: "   ", initial: decimal.NewFromInt(1), want: models.ErrOwnerRequired},
		{name: "negative initial", owner: "Alice", initial: decimal.NewFromInt(-1), want: models.ErrInvalidAmount},
		{name: "too many decimals", owner: "Alice", initial: decimal.RequireFromString("1.001"), want: models.ErrInvalidAmount},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := bank.CreateAccount(ctx, tc.owner, tc.initial); !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestDepositAndWithdraw(t *testing.T) {
	bank, ctx := newBank(t)
	account, err := bank.CreateAccount(ctx, "Alice", decimal.NewFromInt(10))
	if err != nil {
		t.Fatalf("create account: %v", err)
	}

	deposit, err := bank.Deposit(ctx, account.ID, decimal.RequireFromString("25.50"))
	if err != nil {
		t.Fatalf("deposit: %v", err)
	}
	if !deposit.BalanceAfter.Equal(decimal.RequireFromString("35.50")) {
		t.Fatalf("balance after deposit = %s, want 35.50", deposit.BalanceAfter)
	}

	withdrawal, err := bank.Withdraw(ctx, account.ID, decimal.RequireFromString("5.25"))
	if err != nil {
		t.Fatalf("withdraw: %v", err)
	}
	if !withdrawal.BalanceAfter.Equal(decimal.RequireFromString("30.25")) {
		t.Fatalf("balance after withdrawal = %s, want 30.25", withdrawal.BalanceAfter)
	}

	balance, err := bank.Balance(ctx, account.ID)
	if err != nil {
		t.Fatalf("balance: %v", err)
	}
	if !balance.Equal(decimal.RequireFromString("30.25")) {
		t.Fatalf("balance = %s, want 30.25", balance)
	}
}

func TestWithdrawRejectsOverdraft(t *testing.T) {
	bank, ctx := newBank(t)
	account, err := bank.CreateAccount(ctx, "Alice", decimal.NewFromInt(10))
	if err != nil {
		t.Fatalf("create account: %v", err)
	}

	if _, err := bank.Withdraw(ctx, account.ID, decimal.NewFromInt(11)); !errors.Is(err, models.ErrInsufficientFunds) {
		t.Fatalf("err = %v, want ErrInsufficientFunds", err)
	}

	balance, err := bank.Balance(ctx, account.ID)
	if err != nil {
		t.Fatalf("balance: %v", err)
	}
	if !balance.Equal(decimal.NewFromInt(10)) {
		t.Fatalf("balance = %s, want 10", balance)
	}
}

func TestDepositRejectsUnknownAccount(t *testing.T) {
	bank, ctx := newBank(t)

	if _, err := bank.Deposit(ctx, 999, decimal.NewFromInt(1)); !errors.Is(err, models.ErrAccountNotFound) {
		t.Fatalf("err = %v, want ErrAccountNotFound", err)
	}
}

func TestAmountValidation(t *testing.T) {
	bank, ctx := newBank(t)
	invalid := []decimal.Decimal{
		decimal.Zero,
		decimal.NewFromInt(-1),
		decimal.RequireFromString("0.001"),
	}

	for _, amount := range invalid {
		if _, err := bank.Deposit(ctx, 1, amount); !errors.Is(err, models.ErrInvalidAmount) {
			t.Fatalf("deposit %s: err = %v, want ErrInvalidAmount", amount, err)
		}
		if _, err := bank.Withdraw(ctx, 1, amount); !errors.Is(err, models.ErrInvalidAmount) {
			t.Fatalf("withdraw %s: err = %v, want ErrInvalidAmount", amount, err)
		}
		if _, err := bank.Transfer(ctx, 1, 2, amount); !errors.Is(err, models.ErrInvalidAmount) {
			t.Fatalf("transfer %s: err = %v, want ErrInvalidAmount", amount, err)
		}
	}
}

func TestTransferMovesFundsBetweenAccounts(t *testing.T) {
	bank, ctx := newBank(t)
	alice, err := bank.CreateAccount(ctx, "Alice", decimal.NewFromInt(100))
	if err != nil {
		t.Fatalf("create alice: %v", err)
	}
	bob, err := bank.CreateAccount(ctx, "Bob", decimal.NewFromInt(50))
	if err != nil {
		t.Fatalf("create bob: %v", err)
	}

	transfer, err := bank.Transfer(ctx, alice.ID, bob.ID, decimal.RequireFromString("30.50"))
	if err != nil {
		t.Fatalf("transfer: %v", err)
	}
	if transfer.From.Type != models.TypeTransferOut {
		t.Fatalf("from type = %s, want %s", transfer.From.Type, models.TypeTransferOut)
	}
	if transfer.To.Type != models.TypeTransferIn {
		t.Fatalf("to type = %s, want %s", transfer.To.Type, models.TypeTransferIn)
	}
	if transfer.From.TransferID == nil || transfer.To.TransferID == nil || *transfer.From.TransferID != *transfer.To.TransferID {
		t.Fatalf("transfer legs must share a transfer id")
	}
	if !transfer.From.BalanceAfter.Equal(decimal.RequireFromString("69.50")) {
		t.Fatalf("from balance = %s, want 69.50", transfer.From.BalanceAfter)
	}
	if !transfer.To.BalanceAfter.Equal(decimal.RequireFromString("80.50")) {
		t.Fatalf("to balance = %s, want 80.50", transfer.To.BalanceAfter)
	}
}

func TestTransferValidation(t *testing.T) {
	bank, ctx := newBank(t)
	alice, err := bank.CreateAccount(ctx, "Alice", decimal.NewFromInt(100))
	if err != nil {
		t.Fatalf("create alice: %v", err)
	}
	bob, err := bank.CreateAccount(ctx, "Bob", decimal.NewFromInt(0))
	if err != nil {
		t.Fatalf("create bob: %v", err)
	}

	if _, err := bank.Transfer(ctx, alice.ID, alice.ID, decimal.NewFromInt(1)); !errors.Is(err, models.ErrSameAccount) {
		t.Fatalf("err = %v, want ErrSameAccount", err)
	}
	if _, err := bank.Transfer(ctx, alice.ID, bob.ID, decimal.NewFromInt(101)); !errors.Is(err, models.ErrInsufficientFunds) {
		t.Fatalf("err = %v, want ErrInsufficientFunds", err)
	}
	if _, err := bank.Transfer(ctx, alice.ID, 999, decimal.NewFromInt(1)); !errors.Is(err, models.ErrAccountNotFound) {
		t.Fatalf("err = %v, want ErrAccountNotFound", err)
	}

	aliceBalance, err := bank.Balance(ctx, alice.ID)
	if err != nil {
		t.Fatalf("alice balance: %v", err)
	}
	if !aliceBalance.Equal(decimal.NewFromInt(100)) {
		t.Fatalf("alice balance = %s, want 100", aliceBalance)
	}
}

func TestConcurrentWithdrawalsNeverOverdraw(t *testing.T) {
	bank, ctx := newBank(t)
	account, err := bank.CreateAccount(ctx, "Alice", decimal.NewFromInt(100))
	if err != nil {
		t.Fatalf("create account: %v", err)
	}

	const workers = 50
	var succeeded int64
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := bank.Withdraw(ctx, account.ID, decimal.NewFromInt(3))
			switch {
			case err == nil:
				atomic.AddInt64(&succeeded, 1)
			case errors.Is(err, models.ErrInsufficientFunds):
			default:
				t.Errorf("unexpected withdraw error: %v", err)
			}
		}()
	}
	wg.Wait()

	if succeeded != 33 {
		t.Fatalf("successful withdrawals = %d, want 33", succeeded)
	}
	balance, err := bank.Balance(ctx, account.ID)
	if err != nil {
		t.Fatalf("balance: %v", err)
	}
	if !balance.Equal(decimal.NewFromInt(1)) {
		t.Fatalf("balance = %s, want 1", balance)
	}
}

func TestConcurrentTransfersPreserveTotalAndDoNotDeadlock(t *testing.T) {
	bank, ctx := newBank(t)
	alice, err := bank.CreateAccount(ctx, "Alice", decimal.NewFromInt(100))
	if err != nil {
		t.Fatalf("create alice: %v", err)
	}
	bob, err := bank.CreateAccount(ctx, "Bob", decimal.NewFromInt(100))
	if err != nil {
		t.Fatalf("create bob: %v", err)
	}

	const workers = 40
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			from, to := alice.ID, bob.ID
			if i%2 == 1 {
				from, to = bob.ID, alice.ID
			}
			if _, err := bank.Transfer(ctx, from, to, decimal.NewFromInt(5)); err != nil {
				t.Errorf("transfer: %v", err)
			}
		}(i)
	}
	wg.Wait()

	aliceBalance, err := bank.Balance(ctx, alice.ID)
	if err != nil {
		t.Fatalf("alice balance: %v", err)
	}
	bobBalance, err := bank.Balance(ctx, bob.ID)
	if err != nil {
		t.Fatalf("bob balance: %v", err)
	}
	if total := aliceBalance.Add(bobBalance); !total.Equal(decimal.NewFromInt(200)) {
		t.Fatalf("total = %s, want 200", total)
	}
}
