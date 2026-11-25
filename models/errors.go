package models

import "errors"

var (
	ErrAccountNotFound   = errors.New("account not found")
	ErrInsufficientFunds = errors.New("insufficient funds")
	ErrInvalidAmount     = errors.New("amount must be positive with at most two decimal places")
	ErrSameAccount       = errors.New("cannot transfer to the same account")
	ErrOwnerRequired     = errors.New("owner is required")
)
