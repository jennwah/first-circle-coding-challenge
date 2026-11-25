package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"

	"github.com/leejennwah/first-circle-challenge/models"
)

type Repository struct {
	db *sqlx.DB
}

type lockedAccount struct {
	ID      int64           `db:"id"`
	Balance decimal.Decimal `db:"balance"`
}

func Open(ctx context.Context, dsn string) (*Repository, error) {
	db, err := sqlx.ConnectContext(ctx, "pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("connect database: %w", err)
	}
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(20)
	db.SetConnMaxLifetime(5 * time.Minute)
	return &Repository{db: db}, nil
}

func (r *Repository) Close() error {
	return r.db.Close()
}

func (r *Repository) CreateAccount(ctx context.Context, owner string, initial decimal.Decimal) (*models.Account, error) {
	var account models.Account
	err := r.inTx(ctx, func(tx *sqlx.Tx) error {
		const query = `INSERT INTO accounts (owner, balance) VALUES ($1, $2) RETURNING id, owner, balance, created_at, updated_at`
		if err := tx.GetContext(ctx, &account, query, owner, initial); err != nil {
			return err
		}
		if initial.IsZero() {
			return nil
		}
		_, err := insertTransaction(ctx, tx, models.Transaction{
			AccountID:    account.ID,
			Type:         models.TypeDeposit,
			Amount:       initial,
			BalanceAfter: initial,
		})
		return err
	})
	if err != nil {
		return nil, err
	}
	return &account, nil
}

func (r *Repository) Balance(ctx context.Context, accountID int64) (decimal.Decimal, error) {
	var balance decimal.Decimal
	err := r.db.GetContext(ctx, &balance, `SELECT balance FROM accounts WHERE id = $1`, accountID)
	if errors.Is(err, sql.ErrNoRows) {
		return decimal.Zero, fmt.Errorf("%w: %d", models.ErrAccountNotFound, accountID)
	}
	if err != nil {
		return decimal.Zero, err
	}
	return balance, nil
}

func (r *Repository) Deposit(ctx context.Context, accountID int64, amount decimal.Decimal) (*models.Transaction, error) {
	var entry *models.Transaction
	err := r.inTx(ctx, func(tx *sqlx.Tx) error {
		var balance decimal.Decimal
		const query = `UPDATE accounts SET balance = balance + $1, updated_at = now() WHERE id = $2 RETURNING balance`
		if err := tx.GetContext(ctx, &balance, query, amount, accountID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("%w: %d", models.ErrAccountNotFound, accountID)
			}
			return err
		}
		var err error
		entry, err = insertTransaction(ctx, tx, models.Transaction{
			AccountID:    accountID,
			Type:         models.TypeDeposit,
			Amount:       amount,
			BalanceAfter: balance,
		})
		return err
	})
	if err != nil {
		return nil, err
	}
	return entry, nil
}

func (r *Repository) Withdraw(ctx context.Context, accountID int64, amount decimal.Decimal) (*models.Transaction, error) {
	var entry *models.Transaction
	err := r.inTx(ctx, func(tx *sqlx.Tx) error {
		var balance decimal.Decimal
		const lock = `SELECT balance FROM accounts WHERE id = $1 FOR UPDATE`
		if err := tx.GetContext(ctx, &balance, lock, accountID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("%w: %d", models.ErrAccountNotFound, accountID)
			}
			return err
		}
		if balance.LessThan(amount) {
			return models.ErrInsufficientFunds
		}

		const update = `UPDATE accounts SET balance = balance - $1, updated_at = now() WHERE id = $2 RETURNING balance`
		if err := tx.GetContext(ctx, &balance, update, amount, accountID); err != nil {
			return err
		}
		var err error
		entry, err = insertTransaction(ctx, tx, models.Transaction{
			AccountID:    accountID,
			Type:         models.TypeWithdrawal,
			Amount:       amount,
			BalanceAfter: balance,
		})
		return err
	})
	if err != nil {
		return nil, err
	}
	return entry, nil
}

func (r *Repository) Transfer(ctx context.Context, fromID, toID int64, amount decimal.Decimal) (*models.Transfer, error) {
	if fromID == toID {
		return nil, models.ErrSameAccount
	}

	transferID := uuid.NewString()
	result := &models.Transfer{ID: transferID}

	err := r.inTx(ctx, func(tx *sqlx.Tx) error {
		const lock = `SELECT id, balance FROM accounts WHERE id IN ($1, $2) ORDER BY id FOR UPDATE`
		var accounts []lockedAccount
		if err := tx.SelectContext(ctx, &accounts, lock, fromID, toID); err != nil {
			return err
		}
		if len(accounts) != 2 {
			return models.ErrAccountNotFound
		}

		balances := make(map[int64]decimal.Decimal, len(accounts))
		for _, account := range accounts {
			balances[account.ID] = account.Balance
		}
		if balances[fromID].LessThan(amount) {
			return models.ErrInsufficientFunds
		}

		var fromBalance, toBalance decimal.Decimal
		const debit = `UPDATE accounts SET balance = balance - $1, updated_at = now() WHERE id = $2 RETURNING balance`
		if err := tx.GetContext(ctx, &fromBalance, debit, amount, fromID); err != nil {
			return err
		}
		const credit = `UPDATE accounts SET balance = balance + $1, updated_at = now() WHERE id = $2 RETURNING balance`
		if err := tx.GetContext(ctx, &toBalance, credit, amount, toID); err != nil {
			return err
		}

		fromEntry, err := insertTransaction(ctx, tx, models.Transaction{
			AccountID:    fromID,
			TransferID:   &transferID,
			Type:         models.TypeTransferOut,
			Amount:       amount,
			BalanceAfter: fromBalance,
		})
		if err != nil {
			return err
		}
		toEntry, err := insertTransaction(ctx, tx, models.Transaction{
			AccountID:    toID,
			TransferID:   &transferID,
			Type:         models.TypeTransferIn,
			Amount:       amount,
			BalanceAfter: toBalance,
		})
		if err != nil {
			return err
		}

		result.From = fromEntry
		result.To = toEntry
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (r *Repository) Transactions(ctx context.Context, accountID int64) ([]models.Transaction, error) {
	var entries []models.Transaction
	const query = `SELECT id, account_id, transfer_id, type, amount, balance_after, created_at FROM transactions WHERE account_id = $1 ORDER BY id`
	if err := r.db.SelectContext(ctx, &entries, query, accountID); err != nil {
		return nil, err
	}
	return entries, nil
}

func (r *Repository) inTx(ctx context.Context, fn func(*sqlx.Tx) error) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}

func insertTransaction(ctx context.Context, tx *sqlx.Tx, entry models.Transaction) (*models.Transaction, error) {
	const query = `INSERT INTO transactions (account_id, transfer_id, type, amount, balance_after) VALUES ($1, $2, $3, $4, $5) RETURNING id, account_id, transfer_id, type, amount, balance_after, created_at`
	var created models.Transaction
	if err := tx.GetContext(ctx, &created, query, entry.AccountID, entry.TransferID, entry.Type, entry.Amount, entry.BalanceAfter); err != nil {
		return nil, err
	}
	return &created, nil
}
