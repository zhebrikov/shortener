package store

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jackc/pgerrcode"
	"github.com/lib/pq"
)

func TestPostgresRegisterUserOK(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery(`INSERT INTO gophermart_x_users`).
		WithArgs("alice", "hash").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("11111111-1111-1111-1111-111111111111"))

	p := NewPostgres(db)
	id, err := p.RegisterUser(context.Background(), "alice", "hash")
	if err != nil {
		t.Fatal(err)
	}
	if id != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("id %q", id)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestPostgresRegisterUserDuplicate(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery(`INSERT INTO gophermart_x_users`).
		WithArgs("alice", "hash").
		WillReturnError(&pq.Error{Code: pgerrcode.UniqueViolation})

	p := NewPostgres(db)
	_, err = p.RegisterUser(context.Background(), "alice", "hash")
	if !errors.Is(err, ErrLoginTaken) {
		t.Fatalf("got %v", err)
	}
	_ = mock.ExpectationsWereMet()
}

func TestPostgresUploadOrderSameUser(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT user_id`).
		WithArgs("12345678903").
		WillReturnRows(sqlmock.NewRows([]string{"user_id"}).AddRow("550e8400-e29b-41d4-a716-446655440000"))
	mock.ExpectCommit()

	p := NewPostgres(db)
	res, err := p.UploadOrder(context.Background(), "550e8400-e29b-41d4-a716-446655440000", "12345678903")
	if err != nil {
		t.Fatal(err)
	}
	if res != UploadAlreadySameUser {
		t.Fatalf("res %v", res)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestPostgresUploadOrderNew(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT user_id`).
		WithArgs("999").
		WillReturnError(sql.ErrNoRows)
	mock.ExpectExec(`INSERT INTO gophermart_orders`).
		WithArgs("550e8400-e29b-41d4-a716-446655440000", "999").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	p := NewPostgres(db)
	res, err := p.UploadOrder(context.Background(), "550e8400-e29b-41d4-a716-446655440000", "999")
	if err != nil {
		t.Fatal(err)
	}
	if res != UploadAccepted {
		t.Fatalf("res %v", res)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
