package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jackc/pgerrcode"
	"github.com/lib/pq"
)

// Postgres implements Storer using tables gophermart_*.
type Postgres struct {
	db *sql.DB
}

// NewPostgres returns a Storer backed by the given database connection.
func NewPostgres(db *sql.DB) *Postgres {
	return &Postgres{db: db}
}

// RegisterUser implements Storer.
func (p *Postgres) RegisterUser(ctx context.Context, login, passwordHash string) (string, error) {
	var id string
	err := p.db.QueryRowContext(ctx,
		`INSERT INTO gophermart_users (login, password_hash) VALUES ($1, $2) RETURNING id::text`,
		login, passwordHash,
	).Scan(&id)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == pgerrcode.UniqueViolation {
			return "", ErrLoginTaken
		}
		return "", err
	}
	return id, nil
}

// GetUserByLogin implements Storer.
func (p *Postgres) GetUserByLogin(ctx context.Context, login string) (userID, passwordHash string, err error) {
	err = p.db.QueryRowContext(ctx,
		`SELECT id::text, password_hash FROM gophermart_users WHERE login = $1`,
		login,
	).Scan(&userID, &passwordHash)
	return userID, passwordHash, err
}

// UploadOrder implements Storer.
func (p *Postgres) UploadOrder(ctx context.Context, userID, number string) (OrderUploadResult, error) {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	var owner string
	err = tx.QueryRowContext(ctx,
		`SELECT user_id::text FROM gophermart_orders WHERE number = $1 FOR UPDATE`,
		number,
	).Scan(&owner)
	if err == nil {
		if owner == userID {
			if err := tx.Commit(); err != nil {
				return 0, err
			}
			return UploadAlreadySameUser, nil
		}
		if err := tx.Commit(); err != nil {
			return 0, err
		}
		return 0, ErrOrderOwnedByOtherUser
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}

	_, err = tx.ExecContext(ctx,
		`INSERT INTO gophermart_orders (user_id, number, status) VALUES ($1::uuid, $2, 'NEW')`,
		userID, number,
	)
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return UploadAccepted, nil
}

// ListUserOrders implements Storer.
func (p *Postgres) ListUserOrders(ctx context.Context, userID string) ([]OrderRow, error) {
	rows, err := p.db.QueryContext(ctx, `
		SELECT number, status, accrual, uploaded_at
		FROM gophermart_orders
		WHERE user_id = $1::uuid
		ORDER BY uploaded_at DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []OrderRow
	for rows.Next() {
		var r OrderRow
		var acc sql.NullFloat64
		if err := rows.Scan(&r.Number, &r.Status, &acc, &r.UploadedAt); err != nil {
			return nil, err
		}
		if acc.Valid {
			v := acc.Float64
			r.Accrual = &v
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// Balance implements Storer.
func (p *Postgres) Balance(ctx context.Context, userID string) (current, withdrawn float64, err error) {
	err = p.db.QueryRowContext(ctx, `
		SELECT
			COALESCE((
				SELECT SUM(accrual) FROM gophermart_orders
				WHERE user_id = $1::uuid AND status = 'PROCESSED'
			), 0) - COALESCE((
				SELECT SUM(sum) FROM gophermart_withdrawals WHERE user_id = $1::uuid
			), 0),
			COALESCE((
				SELECT SUM(sum) FROM gophermart_withdrawals WHERE user_id = $1::uuid
			), 0)`,
		userID,
	).Scan(&current, &withdrawn)
	return current, withdrawn, err
}

// Withdraw implements Storer.
func (p *Postgres) Withdraw(ctx context.Context, userID, orderNumber string, sum float64) error {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1::text))`, userID)
	if err != nil {
		return err
	}

	var accrued, spent sql.NullFloat64
	err = tx.QueryRowContext(ctx, `
		SELECT
			COALESCE((SELECT SUM(accrual) FROM gophermart_orders WHERE user_id = $1::uuid AND status = 'PROCESSED'), 0),
			COALESCE((SELECT SUM(sum) FROM gophermart_withdrawals WHERE user_id = $1::uuid), 0)`,
		userID,
	).Scan(&accrued, &spent)
	if err != nil {
		return err
	}
	available := accrued.Float64 - spent.Float64
	if available+1e-9 < sum {
		return ErrInsufficientFunds
	}

	_, err = tx.ExecContext(ctx,
		`INSERT INTO gophermart_withdrawals (user_id, order_number, sum) VALUES ($1::uuid, $2, $3)`,
		userID, orderNumber, sum,
	)
	if err != nil {
		return err
	}
	return tx.Commit()
}

// ListWithdrawals implements Storer.
func (p *Postgres) ListWithdrawals(ctx context.Context, userID string) ([]WithdrawalRow, error) {
	rows, err := p.db.QueryContext(ctx, `
		SELECT order_number, sum, processed_at
		FROM gophermart_withdrawals
		WHERE user_id = $1::uuid
		ORDER BY processed_at DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []WithdrawalRow
	for rows.Next() {
		var w WithdrawalRow
		if err := rows.Scan(&w.OrderNumber, &w.Sum, &w.ProcessedAt); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

// PendingAccrualJobs implements Storer.
func (p *Postgres) PendingAccrualJobs(ctx context.Context, limit int) ([]AccrualJob, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := p.db.QueryContext(ctx, `
		SELECT id, number FROM gophermart_orders
		WHERE status IN ('NEW', 'PROCESSING')
		ORDER BY uploaded_at ASC
		LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []AccrualJob
	for rows.Next() {
		var j AccrualJob
		if err := rows.Scan(&j.ID, &j.Number); err != nil {
			return nil, err
		}
		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}

// SyncAccrualStatus implements Storer.
func (p *Postgres) SyncAccrualStatus(ctx context.Context, orderID int64, status string, accrual *float64) error {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var cur string
	err = tx.QueryRowContext(ctx,
		`SELECT status FROM gophermart_orders WHERE id = $1 FOR UPDATE`,
		orderID,
	).Scan(&cur)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if cur == "PROCESSED" || cur == "INVALID" {
		return tx.Commit()
	}

	switch status {
	case "PROCESSING":
		_, err = tx.ExecContext(ctx,
			`UPDATE gophermart_orders SET status = 'PROCESSING' WHERE id = $1`,
			orderID,
		)
	case "INVALID":
		_, err = tx.ExecContext(ctx,
			`UPDATE gophermart_orders SET status = 'INVALID', accrual = NULL WHERE id = $1`,
			orderID,
		)
	case "PROCESSED":
		var acc interface{}
		if accrual != nil {
			acc = *accrual
		}
		_, err = tx.ExecContext(ctx,
			`UPDATE gophermart_orders SET status = 'PROCESSED', accrual = $2 WHERE id = $1`,
			orderID, acc,
		)
	default:
		return fmt.Errorf("unknown accrual sync status %q", status)
	}
	if err != nil {
		return err
	}
	return tx.Commit()
}
