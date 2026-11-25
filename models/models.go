package models

import (
	"time"

	"github.com/shopspring/decimal"
)

type TransactionType string

const (
	TypeDeposit     TransactionType = "deposit"
	TypeWithdrawal  TransactionType = "withdrawal"
	TypeTransferIn  TransactionType = "transfer_in"
	TypeTransferOut TransactionType = "transfer_out"
)

type Account struct {
	ID        int64           `db:"id" json:"id"`
	Owner     string          `db:"owner" json:"owner"`
	Balance   decimal.Decimal `db:"balance" json:"balance"`
	CreatedAt time.Time       `db:"created_at" json:"created_at"`
	UpdatedAt time.Time       `db:"updated_at" json:"updated_at"`
}

type Transaction struct {
	ID           int64           `db:"id" json:"id"`
	AccountID    int64           `db:"account_id" json:"account_id"`
	TransferID   *string         `db:"transfer_id" json:"transfer_id,omitempty"`
	Type         TransactionType `db:"type" json:"type"`
	Amount       decimal.Decimal `db:"amount" json:"amount"`
	BalanceAfter decimal.Decimal `db:"balance_after" json:"balance_after"`
	CreatedAt    time.Time       `db:"created_at" json:"created_at"`
}

type Transfer struct {
	ID   string       `json:"id"`
	From *Transaction `json:"from"`
	To   *Transaction `json:"to"`
}
