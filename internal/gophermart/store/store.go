// Package store defines persistence for users, loyalty orders, and withdrawals backed by PostgreSQL.
package store

import (
	"context"
	"time"
)

// OrderUploadResult describes the outcome of accepting an order number for a user.
type OrderUploadResult int

const (
	// UploadAccepted means a new order row was inserted for asynchronous accrual processing.
	UploadAccepted OrderUploadResult = iota
	// UploadAlreadySameUser means the user had already submitted this order number; the request is idempotent.
	UploadAlreadySameUser
)

// OrderRow is one element of GET /api/user/orders.
type OrderRow struct {
	Number     string
	Status     string
	Accrual    *float64
	UploadedAt time.Time
}

// WithdrawalRow is one element of GET /api/user/withdrawals.
type WithdrawalRow struct {
	OrderNumber string
	Sum         float64
	ProcessedAt time.Time
}

// AccrualJob identifies an order row that should be synchronized with the accrual system.
type AccrualJob struct {
	ID     int64
	Number string
}

// Storer is implemented by PostgreSQL-backed storage for the Gophermart API.
type Storer interface {
	// RegisterUser inserts a new user with the given bcrypt password hash.
	RegisterUser(ctx context.Context, login, passwordHash string) (userID string, err error)
	// GetUserByLogin returns the user id and password hash for a login, or sql.ErrNoRows.
	GetUserByLogin(ctx context.Context, login string) (userID, passwordHash string, err error)
	// UploadOrder inserts a NEW order or reports idempotency / conflict.
	UploadOrder(ctx context.Context, userID, number string) (OrderUploadResult, error)
	// ListUserOrders returns orders sorted by upload time descending (newest first).
	ListUserOrders(ctx context.Context, userID string) ([]OrderRow, error)
	// Balance returns current available points and lifetime withdrawn points.
	Balance(ctx context.Context, userID string) (current, withdrawn float64, err error)
	// Withdraw records a withdrawal if the user has enough balance (Luhn-valid order is enforced by handler).
	Withdraw(ctx context.Context, userID, orderNumber string, sum float64) error
	// ListWithdrawals returns withdrawals sorted by processed time descending.
	ListWithdrawals(ctx context.Context, userID string) ([]WithdrawalRow, error)
	// PendingAccrualJobs returns orders that still need accrual polling.
	PendingAccrualJobs(ctx context.Context, limit int) ([]AccrualJob, error)
	// SyncAccrualStatus updates local order state from the accrual system response.
	SyncAccrualStatus(ctx context.Context, orderID int64, status string, accrual *float64) error
}
