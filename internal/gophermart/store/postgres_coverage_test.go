package store

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestPostgresRegisterUserDBError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery(`INSERT INTO gophermart_users`).
		WithArgs("a", "h").
		WillReturnError(errors.New("db down"))

	p := NewPostgres(db)
	_, err = p.RegisterUser(context.Background(), "a", "h")
	if err == nil || err.Error() != "db down" {
		t.Fatalf("got %v", err)
	}
	_ = mock.ExpectationsWereMet()
}

func TestPostgresGetUserByLogin(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery(`SELECT id::text, password_hash`).
		WithArgs("alice").
		WillReturnRows(sqlmock.NewRows([]string{"id", "password_hash"}).AddRow("uid-1", "hash"))

	p := NewPostgres(db)
	uid, hash, err := p.GetUserByLogin(context.Background(), "alice")
	if err != nil {
		t.Fatal(err)
	}
	if uid != "uid-1" || hash != "hash" {
		t.Fatalf("uid %q hash %q", uid, hash)
	}
	_ = mock.ExpectationsWereMet()
}

func TestPostgresUploadOrderBeginError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin().WillReturnError(errors.New("begin"))

	p := NewPostgres(db)
	_, err = p.UploadOrder(context.Background(), "550e8400-e29b-41d4-a716-446655440000", "1")
	if err == nil || err.Error() != "begin" {
		t.Fatalf("got %v", err)
	}
}

func TestPostgresUploadOrderOtherUser(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT user_id`).
		WithArgs("12345678903").
		WillReturnRows(sqlmock.NewRows([]string{"user_id"}).AddRow("11111111-1111-1111-1111-111111111111"))
	mock.ExpectCommit()

	p := NewPostgres(db)
	_, err = p.UploadOrder(context.Background(), "550e8400-e29b-41d4-a716-446655440000", "12345678903")
	if !errors.Is(err, ErrOrderOwnedByOtherUser) {
		t.Fatalf("got %v", err)
	}
}

func TestPostgresUploadOrderQueryError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT user_id`).WillReturnError(errors.New("query"))
	p := NewPostgres(db)
	_, err = p.UploadOrder(context.Background(), "550e8400-e29b-41d4-a716-446655440000", "1")
	if err == nil || err.Error() != "query" {
		t.Fatalf("got %v", err)
	}
}

func TestPostgresUploadOrderSameUserCommitError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	uid := "550e8400-e29b-41d4-a716-446655440000"
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT user_id`).
		WithArgs("12345678903").
		WillReturnRows(sqlmock.NewRows([]string{"user_id"}).AddRow(uid))
	mock.ExpectCommit().WillReturnError(errors.New("commit"))

	p := NewPostgres(db)
	_, err = p.UploadOrder(context.Background(), uid, "12345678903")
	if err == nil || err.Error() != "commit" {
		t.Fatalf("got %v", err)
	}
}

func TestPostgresUploadOrderInsertError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	uid := "550e8400-e29b-41d4-a716-446655440000"
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT user_id`).
		WithArgs("999").
		WillReturnError(sql.ErrNoRows)
	mock.ExpectExec(`INSERT INTO gophermart_orders`).
		WithArgs(uid, "999").
		WillReturnError(errors.New("insert"))
	p := NewPostgres(db)
	_, err = p.UploadOrder(context.Background(), uid, "999")
	if err == nil || err.Error() != "insert" {
		t.Fatalf("got %v", err)
	}
}

func TestPostgresUploadOrderCommitAfterInsertError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	uid := "550e8400-e29b-41d4-a716-446655440000"
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT user_id`).
		WithArgs("999").
		WillReturnError(sql.ErrNoRows)
	mock.ExpectExec(`INSERT INTO gophermart_orders`).
		WithArgs(uid, "999").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit().WillReturnError(errors.New("commit"))

	p := NewPostgres(db)
	_, err = p.UploadOrder(context.Background(), uid, "999")
	if err == nil || err.Error() != "commit" {
		t.Fatalf("got %v", err)
	}
}

func TestPostgresUploadOrderOtherUserCommitError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT user_id`).
		WithArgs("12345678903").
		WillReturnRows(sqlmock.NewRows([]string{"user_id"}).AddRow("11111111-1111-1111-1111-111111111111"))
	mock.ExpectCommit().WillReturnError(errors.New("commit"))

	p := NewPostgres(db)
	_, err = p.UploadOrder(context.Background(), "550e8400-e29b-41d4-a716-446655440000", "12345678903")
	if err == nil || err.Error() != "commit" {
		t.Fatalf("got %v", err)
	}
}

func TestPostgresListUserOrdersEmpty(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	uid := "550e8400-e29b-41d4-a716-446655440000"
	mock.ExpectQuery(`SELECT number, status, accrual, uploaded_at`).
		WithArgs(uid).
		WillReturnRows(sqlmock.NewRows([]string{"number", "status", "accrual", "uploaded_at"}))

	p := NewPostgres(db)
	var got []OrderRow
	for row, err := range p.ListUserOrders(context.Background(), uid) {
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, row)
	}
	if len(got) != 0 {
		t.Fatalf("len %d", len(got))
	}
}

func TestPostgresListUserOrdersWithAccrual(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	uid := "550e8400-e29b-41d4-a716-446655440000"
	ts := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	mock.ExpectQuery(`SELECT number, status, accrual, uploaded_at`).
		WithArgs(uid).
		WillReturnRows(sqlmock.NewRows([]string{"number", "status", "accrual", "uploaded_at"}).
			AddRow("1", "PROCESSED", 10.5, ts))

	p := NewPostgres(db)
	var got []OrderRow
	for row, err := range p.ListUserOrders(context.Background(), uid) {
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, row)
	}
	if len(got) != 1 || got[0].Number != "1" || got[0].Accrual == nil || *got[0].Accrual != 10.5 {
		t.Fatalf("%+v", got)
	}
}

func TestPostgresListUserOrdersScanError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	uid := "550e8400-e29b-41d4-a716-446655440000"
	ts := time.Now()
	rows := sqlmock.NewRows([]string{"number", "status", "accrual", "uploaded_at"}).
		AddRow("1", "NEW", nil, ts).
		AddRow(nil, nil, nil, nil).
		RowError(1, errors.New("scan"))
	mock.ExpectQuery(`SELECT number, status, accrual, uploaded_at`).
		WithArgs(uid).
		WillReturnRows(rows)

	p := NewPostgres(db)
	var iterErr error
	for _, err := range p.ListUserOrders(context.Background(), uid) {
		if err != nil {
			iterErr = err
			break
		}
	}
	if iterErr == nil || iterErr.Error() != "scan" {
		t.Fatalf("got %v", iterErr)
	}
}

func TestPostgresListUserOrdersRowsErr(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	uid := "550e8400-e29b-41d4-a716-446655440000"
	ts := time.Now()
	rows := sqlmock.NewRows([]string{"number", "status", "accrual", "uploaded_at"}).
		AddRow("1", "NEW", nil, ts).
		RowError(0, errors.New("rows err"))

	mock.ExpectQuery(`SELECT number, status, accrual, uploaded_at`).
		WithArgs(uid).
		WillReturnRows(rows)

	p := NewPostgres(db)
	var iterErr error
	for _, err := range p.ListUserOrders(context.Background(), uid) {
		if err != nil {
			iterErr = err
			break
		}
	}
	if iterErr == nil || iterErr.Error() != "rows err" {
		t.Fatalf("got %v", iterErr)
	}
}

func TestPostgresListUserOrdersQueryError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery(`SELECT number, status, accrual, uploaded_at`).WillReturnError(errors.New("q"))
	p := NewPostgres(db)
	var iterErr error
	for _, err := range p.ListUserOrders(context.Background(), "550e8400-e29b-41d4-a716-446655440000") {
		if err != nil {
			iterErr = err
			break
		}
	}
	if iterErr == nil || iterErr.Error() != "q" {
		t.Fatalf("got %v", iterErr)
	}
}

func TestPostgresBalance(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	uid := "550e8400-e29b-41d4-a716-446655440000"
	mock.ExpectQuery(`SELECT`).
		WithArgs(uid).
		WillReturnRows(sqlmock.NewRows([]string{"current", "withdrawn"}).AddRow(100.0, 20.0))

	p := NewPostgres(db)
	cur, w, err := p.Balance(context.Background(), uid)
	if err != nil {
		t.Fatal(err)
	}
	if cur != 100 || w != 20 {
		t.Fatalf("%v %v", cur, w)
	}
}

func TestPostgresBalanceError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery(`SELECT`).WillReturnError(errors.New("q"))
	p := NewPostgres(db)
	_, _, err = p.Balance(context.Background(), "550e8400-e29b-41d4-a716-446655440000")
	if err == nil {
		t.Fatal("expected err")
	}
}

func TestPostgresWithdrawOK(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	uid := "550e8400-e29b-41d4-a716-446655440000"
	mock.ExpectBegin()
	mock.ExpectExec(`SELECT pg_advisory_xact_lock`).WithArgs(uid).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(`SELECT`).
		WithArgs(uid).
		WillReturnRows(sqlmock.NewRows([]string{"acc", "spent"}).AddRow(100.0, 0.0))
	mock.ExpectExec(`INSERT INTO gophermart_withdrawals`).
		WithArgs(uid, "12345678903", 10.0).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	p := NewPostgres(db)
	err = p.Withdraw(context.Background(), uid, "12345678903", 10)
	if err != nil {
		t.Fatal(err)
	}
}

func TestPostgresWithdrawBeginError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin().WillReturnError(errors.New("begin"))
	p := NewPostgres(db)
	err = p.Withdraw(context.Background(), "550e8400-e29b-41d4-a716-446655440000", "12345678903", 1)
	if err == nil || err.Error() != "begin" {
		t.Fatalf("got %v", err)
	}
}

func TestPostgresWithdrawBalanceQueryError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	uid := "550e8400-e29b-41d4-a716-446655440000"
	mock.ExpectBegin()
	mock.ExpectExec(`SELECT pg_advisory_xact_lock`).WithArgs(uid).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(`SELECT`).
		WithArgs(uid).
		WillReturnError(errors.New("bal"))

	p := NewPostgres(db)
	err = p.Withdraw(context.Background(), uid, "12345678903", 1)
	if err == nil || err.Error() != "bal" {
		t.Fatalf("got %v", err)
	}
}

func TestPostgresWithdrawLockError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	uid := "550e8400-e29b-41d4-a716-446655440000"
	mock.ExpectBegin()
	mock.ExpectExec(`SELECT pg_advisory_xact_lock`).WillReturnError(errors.New("lock"))
	p := NewPostgres(db)
	err = p.Withdraw(context.Background(), uid, "12345678903", 1)
	if err == nil || err.Error() != "lock" {
		t.Fatalf("got %v", err)
	}
}

func TestPostgresWithdrawInsufficient(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	uid := "550e8400-e29b-41d4-a716-446655440000"
	mock.ExpectBegin()
	mock.ExpectExec(`SELECT pg_advisory_xact_lock`).WithArgs(uid).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(`SELECT`).
		WithArgs(uid).
		WillReturnRows(sqlmock.NewRows([]string{"acc", "spent"}).AddRow(1.0, 0.0))

	p := NewPostgres(db)
	err = p.Withdraw(context.Background(), uid, "12345678903", 999)
	if !errors.Is(err, ErrInsufficientFunds) {
		t.Fatalf("got %v", err)
	}
}

func TestPostgresWithdrawInsertError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	uid := "550e8400-e29b-41d4-a716-446655440000"
	mock.ExpectBegin()
	mock.ExpectExec(`SELECT pg_advisory_xact_lock`).WithArgs(uid).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(`SELECT`).
		WithArgs(uid).
		WillReturnRows(sqlmock.NewRows([]string{"acc", "spent"}).AddRow(100.0, 0.0))
	mock.ExpectExec(`INSERT INTO gophermart_withdrawals`).
		WillReturnError(errors.New("ins"))

	p := NewPostgres(db)
	err = p.Withdraw(context.Background(), uid, "12345678903", 10)
	if err == nil || err.Error() != "ins" {
		t.Fatalf("got %v", err)
	}
}

func TestPostgresListWithdrawalsEmpty(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	uid := "550e8400-e29b-41d4-a716-446655440000"
	mock.ExpectQuery(`SELECT order_number, sum, processed_at`).
		WithArgs(uid).
		WillReturnRows(sqlmock.NewRows([]string{"order_number", "sum", "processed_at"}))

	p := NewPostgres(db)
	rows, err := p.ListWithdrawals(context.Background(), uid)
	if err != nil || len(rows) != 0 {
		t.Fatalf("%v %+v", err, rows)
	}
}

func TestPostgresListWithdrawalsQueryError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery(`SELECT order_number, sum, processed_at`).WillReturnError(errors.New("q"))
	p := NewPostgres(db)
	_, err = p.ListWithdrawals(context.Background(), "550e8400-e29b-41d4-a716-446655440000")
	if err == nil {
		t.Fatal("expected err")
	}
}

func TestPostgresListWithdrawalsScanError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	uid := "550e8400-e29b-41d4-a716-446655440000"
	rows := sqlmock.NewRows([]string{"order_number", "sum", "processed_at"}).
		AddRow("1", 1.0, time.Now()).
		AddRow(nil, nil, nil).
		RowError(1, errors.New("scan"))
	mock.ExpectQuery(`SELECT order_number, sum, processed_at`).
		WithArgs(uid).
		WillReturnRows(rows)

	p := NewPostgres(db)
	_, err = p.ListWithdrawals(context.Background(), uid)
	if err == nil {
		t.Fatal("expected err")
	}
}

func TestPostgresListWithdrawalsRowsErr(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	uid := "550e8400-e29b-41d4-a716-446655440000"
	rows := sqlmock.NewRows([]string{"order_number", "sum", "processed_at"}).
		AddRow("1", 1.0, time.Now()).
		RowError(0, errors.New("walk"))
	mock.ExpectQuery(`SELECT order_number, sum, processed_at`).
		WithArgs(uid).
		WillReturnRows(rows)

	p := NewPostgres(db)
	_, err = p.ListWithdrawals(context.Background(), uid)
	if err == nil {
		t.Fatal("expected err")
	}
}

func TestPostgresListWithdrawalsRows(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	uid := "550e8400-e29b-41d4-a716-446655440000"
	ts := time.Date(2021, 2, 3, 4, 5, 6, 0, time.UTC)
	mock.ExpectQuery(`SELECT order_number, sum, processed_at`).
		WithArgs(uid).
		WillReturnRows(sqlmock.NewRows([]string{"order_number", "sum", "processed_at"}).
			AddRow("12345678903", 5.5, ts))

	p := NewPostgres(db)
	rows, err := p.ListWithdrawals(context.Background(), uid)
	if err != nil || len(rows) != 1 || rows[0].OrderNumber != "12345678903" {
		t.Fatalf("%v %+v", err, rows)
	}
}

func TestPostgresPendingAccrualJobsLimitDefault(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery(`SELECT id, number FROM gophermart_orders`).
		WithArgs(10).
		WillReturnRows(sqlmock.NewRows([]string{"id", "number"}))

	p := NewPostgres(db)
	jobs, err := p.PendingAccrualJobs(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 0 {
		t.Fatalf("%+v", jobs)
	}
}

func TestPostgresPendingAccrualJobsQueryError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery(`SELECT id, number FROM gophermart_orders`).WillReturnError(errors.New("q"))
	p := NewPostgres(db)
	_, err = p.PendingAccrualJobs(context.Background(), 3)
	if err == nil {
		t.Fatal("expected err")
	}
}

func TestPostgresPendingAccrualJobsScanError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	rows := sqlmock.NewRows([]string{"id", "number"}).
		AddRow(1, "1").
		AddRow(nil, nil).
		RowError(1, errors.New("scan"))
	mock.ExpectQuery(`SELECT id, number FROM gophermart_orders`).
		WithArgs(5).
		WillReturnRows(rows)

	p := NewPostgres(db)
	_, err = p.PendingAccrualJobs(context.Background(), 5)
	if err == nil {
		t.Fatal("expected err")
	}
}

func TestPostgresPendingAccrualJobsRowsErr(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	rows := sqlmock.NewRows([]string{"id", "number"}).
		AddRow(1, "1").
		RowError(0, errors.New("walk"))
	mock.ExpectQuery(`SELECT id, number FROM gophermart_orders`).
		WithArgs(5).
		WillReturnRows(rows)

	p := NewPostgres(db)
	_, err = p.PendingAccrualJobs(context.Background(), 5)
	if err == nil {
		t.Fatal("expected err")
	}
}

func TestPostgresPendingAccrualJobsRows(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery(`SELECT id, number FROM gophermart_orders`).
		WithArgs(5).
		WillReturnRows(sqlmock.NewRows([]string{"id", "number"}).AddRow(1, "12345678903"))

	p := NewPostgres(db)
	jobs, err := p.PendingAccrualJobs(context.Background(), 5)
	if err != nil || len(jobs) != 1 || jobs[0].ID != 1 {
		t.Fatalf("%v %+v", err, jobs)
	}
}

func TestPostgresSyncAccrualStatusNoRows(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT status FROM gophermart_orders`).
		WithArgs(int64(9)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectRollback()

	p := NewPostgres(db)
	err = p.SyncAccrualStatus(context.Background(), 9, "PROCESSED", nil)
	if err != nil {
		t.Fatal(err)
	}
}

func TestPostgresSyncAccrualStatusSkipTerminal(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT status FROM gophermart_orders`).
		WithArgs(int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("PROCESSED"))
	mock.ExpectCommit()

	p := NewPostgres(db)
	err = p.SyncAccrualStatus(context.Background(), 9, "PROCESSING", nil)
	if err != nil {
		t.Fatal(err)
	}
}

func TestPostgresSyncAccrualStatusProcessing(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT status FROM gophermart_orders`).
		WithArgs(int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("NEW"))
	mock.ExpectExec(`UPDATE gophermart_orders SET status = 'PROCESSING'`).
		WithArgs(int64(9)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	p := NewPostgres(db)
	err = p.SyncAccrualStatus(context.Background(), 9, "PROCESSING", nil)
	if err != nil {
		t.Fatal(err)
	}
}

func TestPostgresSyncAccrualStatusInvalid(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT status FROM gophermart_orders`).
		WithArgs(int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("NEW"))
	mock.ExpectExec(`UPDATE gophermart_orders SET status = 'INVALID'`).
		WithArgs(int64(9)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	p := NewPostgres(db)
	err = p.SyncAccrualStatus(context.Background(), 9, "INVALID", nil)
	if err != nil {
		t.Fatal(err)
	}
}

func TestPostgresSyncAccrualStatusProcessed(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	v := 42.0
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT status FROM gophermart_orders`).
		WithArgs(int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("NEW"))
	mock.ExpectExec(`UPDATE gophermart_orders SET status = 'PROCESSED'`).
		WithArgs(int64(9), v).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	p := NewPostgres(db)
	err = p.SyncAccrualStatus(context.Background(), 9, "PROCESSED", &v)
	if err != nil {
		t.Fatal(err)
	}
}

func TestPostgresSyncSkipWhenInvalid(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT status FROM gophermart_orders`).
		WithArgs(int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("INVALID"))
	mock.ExpectCommit()

	p := NewPostgres(db)
	err = p.SyncAccrualStatus(context.Background(), 9, "PROCESSING", nil)
	if err != nil {
		t.Fatal(err)
	}
}

func TestPostgresSyncAccrualStatusProcessedNilAccrual(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT status FROM gophermart_orders`).
		WithArgs(int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("NEW"))
	mock.ExpectExec(`UPDATE gophermart_orders SET status = 'PROCESSED'`).
		WithArgs(int64(9), nil).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	p := NewPostgres(db)
	err = p.SyncAccrualStatus(context.Background(), 9, "PROCESSED", nil)
	if err != nil {
		t.Fatal(err)
	}
}

func TestPostgresSyncAccrualStatusUnknownSwitch(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT status FROM gophermart_orders`).
		WithArgs(int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("NEW"))

	p := NewPostgres(db)
	err = p.SyncAccrualStatus(context.Background(), 9, "WEIRD", nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestPostgresSyncBeginError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin().WillReturnError(errors.New("begin"))
	p := NewPostgres(db)
	err = p.SyncAccrualStatus(context.Background(), 1, "PROCESSING", nil)
	if err == nil || err.Error() != "begin" {
		t.Fatalf("got %v", err)
	}
}

func TestPostgresSyncSelectError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT status FROM gophermart_orders`).WillReturnError(errors.New("sel"))
	p := NewPostgres(db)
	err = p.SyncAccrualStatus(context.Background(), 1, "PROCESSING", nil)
	if err == nil || err.Error() != "sel" {
		t.Fatalf("got %v", err)
	}
}

func TestPostgresSyncCommitFinalError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT status FROM gophermart_orders`).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("NEW"))
	mock.ExpectExec(`UPDATE gophermart_orders SET status = 'PROCESSING'`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit().WillReturnError(errors.New("commit"))

	p := NewPostgres(db)
	err = p.SyncAccrualStatus(context.Background(), 1, "PROCESSING", nil)
	if err == nil || err.Error() != "commit" {
		t.Fatalf("got %v", err)
	}
}

func TestPostgresSyncUpdateError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT status FROM gophermart_orders`).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("NEW"))
	mock.ExpectExec(`UPDATE gophermart_orders SET status = 'PROCESSING'`).
		WillReturnError(errors.New("upd"))

	p := NewPostgres(db)
	err = p.SyncAccrualStatus(context.Background(), 1, "PROCESSING", nil)
	if err == nil || err.Error() != "upd" {
		t.Fatalf("got %v", err)
	}
}
