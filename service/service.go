package service

import (
	"context"
	"strings"

	"github.com/shopspring/decimal"

	"github.com/leejennwah/first-circle-challenge/models"
)

type Bank interface {
	CreateAccount(ctx context.Context, owner string, initial decimal.Decimal) (*models.Account, error)
	Balance(ctx context.Context, accountID int64) (decimal.Decimal, error)
	Deposit(ctx context.Context, accountID int64, amount decimal.Decimal) (*models.Transaction, error)
	Withdraw(ctx context.Context, accountID int64, amount decimal.Decimal) (*models.Transaction, error)
	Transfer(ctx context.Context, fromID, toID int64, amount decimal.Decimal) (*models.Transfer, error)
	Transactions(ctx context.Context, accountID int64) ([]models.Transaction, error)
}

type Repository interface {
	CreateAccount(ctx context.Context, owner string, initial decimal.Decimal) (*models.Account, error)
	Balance(ctx context.Context, accountID int64) (decimal.Decimal, error)
	Deposit(ctx context.Context, accountID int64, amount decimal.Decimal) (*models.Transaction, error)
	Withdraw(ctx context.Context, accountID int64, amount decimal.Decimal) (*models.Transaction, error)
	Transfer(ctx context.Context, fromID, toID int64, amount decimal.Decimal) (*models.Transfer, error)
	Transactions(ctx context.Context, accountID int64) ([]models.Transaction, error)
}

type Service struct {
	repository Repository
}

func New(repository Repository) *Service {
	return &Service{repository: repository}
}

func (s *Service) CreateAccount(ctx context.Context, owner string, initial decimal.Decimal) (*models.Account, error) {
	owner = strings.TrimSpace(owner)
	if owner == "" {
		return nil, models.ErrOwnerRequired
	}
	if initial.IsNegative() || !initial.Equal(initial.Round(2)) {
		return nil, models.ErrInvalidAmount
	}
	return s.repository.CreateAccount(ctx, owner, initial)
}

func (s *Service) Balance(ctx context.Context, accountID int64) (decimal.Decimal, error) {
	return s.repository.Balance(ctx, accountID)
}

func (s *Service) Deposit(ctx context.Context, accountID int64, amount decimal.Decimal) (*models.Transaction, error) {
	if err := validateAmount(amount); err != nil {
		return nil, err
	}
	return s.repository.Deposit(ctx, accountID, amount)
}

func (s *Service) Withdraw(ctx context.Context, accountID int64, amount decimal.Decimal) (*models.Transaction, error) {
	if err := validateAmount(amount); err != nil {
		return nil, err
	}
	return s.repository.Withdraw(ctx, accountID, amount)
}

func (s *Service) Transfer(ctx context.Context, fromID, toID int64, amount decimal.Decimal) (*models.Transfer, error) {
	if err := validateAmount(amount); err != nil {
		return nil, err
	}
	if fromID == toID {
		return nil, models.ErrSameAccount
	}
	return s.repository.Transfer(ctx, fromID, toID, amount)
}

func (s *Service) Transactions(ctx context.Context, accountID int64) ([]models.Transaction, error) {
	return s.repository.Transactions(ctx, accountID)
}

func validateAmount(amount decimal.Decimal) error {
	if !amount.IsPositive() || !amount.Equal(amount.Round(2)) {
		return models.ErrInvalidAmount
	}
	return nil
}
