package storage

import (
	"database/sql"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jackc/pgerrcode"
	"github.com/lib/pq"
)

func TestNewPostgresStorage(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() err = %v", err)
	}
	defer db.Close()

	ps := NewPostgresStorage(db)
	if ps == nil {
		t.Fatal("NewPostgresStorage returned nil")
	}
	if ps.db != db {
		t.Error("NewPostgresStorage did not set db")
	}
}

func TestPostgresStorage_WriteStorage(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() err = %v", err)
	}
	defer db.Close()

	ps := NewPostgresStorage(db)
	link := Link{UUID: 1, ShortURL: "short1", OriginalURL: "https://example.com/one"}

	mock.ExpectExec("INSERT INTO links \\(url, short_url, user_id\\) VALUES \\(\\$1, \\$2, \\$3\\)").
		WithArgs(link.OriginalURL, link.ShortURL, nil).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := ps.WriteStorage(link); err != nil {
		t.Fatalf("WriteStorage() err = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestPostgresStorage_WriteStorage_Error(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() err = %v", err)
	}
	defer db.Close()

	ps := NewPostgresStorage(db)
	link := Link{UUID: 1, ShortURL: "x", OriginalURL: "https://x.com"}

	mock.ExpectExec("INSERT INTO links \\(url, short_url, user_id\\) VALUES \\(\\$1, \\$2, \\$3\\)").
		WithArgs(link.OriginalURL, link.ShortURL, nil).
		WillReturnError(sql.ErrConnDone)

	if err := ps.WriteStorage(link); err != sql.ErrConnDone {
		t.Errorf("WriteStorage() err = %v, want %v", err, sql.ErrConnDone)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestPostgresStorage_WriteStorage_UniqueViolation(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() err = %v", err)
	}
	defer db.Close()

	ps := NewPostgresStorage(db)
	link := Link{UUID: 1, ShortURL: "x", OriginalURL: "https://x.com"}

	mock.ExpectExec("INSERT INTO links \\(url, short_url, user_id\\) VALUES \\(\\$1, \\$2, \\$3\\)").
		WithArgs(link.OriginalURL, link.ShortURL, nil).
		WillReturnError(&pq.Error{Code: pgerrcode.UniqueViolation})

	err = ps.WriteStorage(link)
	if err == nil {
		t.Fatal("WriteStorage() err = nil, want ErrDuplicateURL")
	}
	if !errors.Is(err, ErrDuplicateURL) {
		t.Errorf("WriteStorage() err = %v, want ErrDuplicateURL", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestPostgresStorage_GetShortURLByOriginalURL(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() err = %v", err)
	}
	defer db.Close()

	ps := NewPostgresStorage(db)
	mock.ExpectQuery("SELECT short_url FROM links WHERE url = \\$1").
		WithArgs("https://example.com").
		WillReturnRows(sqlmock.NewRows([]string{"short_url"}).AddRow("http://localhost/abc12345"))

	shortURL, err := ps.GetShortURLByOriginalURL("https://example.com")
	if err != nil {
		t.Fatalf("GetShortURLByOriginalURL() err = %v", err)
	}
	if shortURL != "http://localhost/abc12345" {
		t.Errorf("GetShortURLByOriginalURL() = %q, want %q", shortURL, "http://localhost/abc12345")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestPostgresStorage_ReadStorage_Empty(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() err = %v", err)
	}
	defer db.Close()

	ps := NewPostgresStorage(db)
	rows := sqlmock.NewRows([]string{"url", "short_url", "user_id"})
	mock.ExpectQuery("SELECT url, short_url, user_id FROM links ORDER BY id").
		WillReturnRows(rows)

	links, err := ps.ReadStorage()
	if err != nil {
		t.Fatalf("ReadStorage() err = %v", err)
	}
	if len(links) != 0 {
		t.Errorf("len(links) = %d, want 0", len(links))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestPostgresStorage_ReadStorage_WithRows(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() err = %v", err)
	}
	defer db.Close()

	ps := NewPostgresStorage(db)
	rows := sqlmock.NewRows([]string{"url", "short_url", "user_id"}).
		AddRow("https://first.com", "f1", nil).
		AddRow("https://second.com", "s2", nil)
	mock.ExpectQuery("SELECT url, short_url, user_id FROM links ORDER BY id").
		WillReturnRows(rows)

	links, err := ps.ReadStorage()
	if err != nil {
		t.Fatalf("ReadStorage() err = %v", err)
	}
	if len(links) != 2 {
		t.Fatalf("len(links) = %d, want 2", len(links))
	}
	if links[0].UUID != 1 || links[0].OriginalURL != "https://first.com" || links[0].ShortURL != "f1" {
		t.Errorf("links[0] = %+v", links[0])
	}
	if links[1].UUID != 2 || links[1].OriginalURL != "https://second.com" || links[1].ShortURL != "s2" {
		t.Errorf("links[1] = %+v", links[1])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestPostgresStorage_ReadStorage_QueryError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() err = %v", err)
	}
	defer db.Close()

	ps := NewPostgresStorage(db)
	mock.ExpectQuery("SELECT url, short_url, user_id FROM links ORDER BY id").
		WillReturnError(sql.ErrNoRows)

	_, err = ps.ReadStorage()
	if err != sql.ErrNoRows {
		t.Errorf("ReadStorage() err = %v, want %v", err, sql.ErrNoRows)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestPostgresStorage_GetByShortURL_Found(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() err = %v", err)
	}
	defer db.Close()

	ps := NewPostgresStorage(db)
	rows := sqlmock.NewRows([]string{"url", "short_url", "user_id"}).
		AddRow("https://example.com/page", "abc123", nil)
	mock.ExpectQuery("SELECT url, short_url, user_id FROM links WHERE short_url = \\$1").
		WithArgs("abc123").
		WillReturnRows(rows)

	link, err := ps.GetByShortURL("abc123")
	if err != nil {
		t.Fatalf("GetByShortURL() err = %v", err)
	}
	if link == nil {
		t.Fatal("GetByShortURL() returned nil link")
	}
	if link.OriginalURL != "https://example.com/page" || link.ShortURL != "abc123" {
		t.Errorf("GetByShortURL() link = %+v", link)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestPostgresStorage_GetByShortURL_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() err = %v", err)
	}
	defer db.Close()

	ps := NewPostgresStorage(db)
	mock.ExpectQuery("SELECT url, short_url, user_id FROM links WHERE short_url = \\$1").
		WithArgs("missing").
		WillReturnError(sql.ErrNoRows)

	link, err := ps.GetByShortURL("missing")
	if err != sql.ErrNoRows {
		t.Errorf("GetByShortURL() err = %v, want sql.ErrNoRows", err)
	}
	if link != nil {
		t.Errorf("GetByShortURL() link = %+v, want nil", link)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestPostgresStorage_GetByShortURL_QueryError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() err = %v", err)
	}
	defer db.Close()

	ps := NewPostgresStorage(db)
	mock.ExpectQuery("SELECT url, short_url, user_id FROM links WHERE short_url = \\$1").
		WithArgs("x").
		WillReturnError(sql.ErrConnDone)

	link, err := ps.GetByShortURL("x")
	if err != sql.ErrConnDone {
		t.Errorf("GetByShortURL() err = %v, want %v", err, sql.ErrConnDone)
	}
	if link != nil {
		t.Errorf("GetByShortURL() link = %+v, want nil", link)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestPostgresStorage_WriteStorageBatch_Empty(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() err = %v", err)
	}
	defer db.Close()

	ps := NewPostgresStorage(db)
	if err := ps.WriteStorageBatch(nil); err != nil {
		t.Errorf("WriteStorageBatch(nil) err = %v", err)
	}
	if err := ps.WriteStorageBatch([]Link{}); err != nil {
		t.Errorf("WriteStorageBatch([]) err = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestPostgresStorage_WriteStorageBatch_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() err = %v", err)
	}
	defer db.Close()

	ps := NewPostgresStorage(db)
	links := []Link{
		{OriginalURL: "https://a.com", ShortURL: "s1"},
		{OriginalURL: "https://b.com", ShortURL: "s2"},
	}

	mock.ExpectExec("INSERT INTO links \\(url, short_url, user_id\\) VALUES \\(\\$1, \\$2, \\$3\\),\\(\\$4, \\$5, \\$6\\)").
		WithArgs("https://a.com", "s1", nil, "https://b.com", "s2", nil).
		WillReturnResult(sqlmock.NewResult(0, 2))

	if err := ps.WriteStorageBatch(links); err != nil {
		t.Fatalf("WriteStorageBatch() err = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestPostgresStorage_WriteStorageBatch_UniqueViolation(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() err = %v", err)
	}
	defer db.Close()

	ps := NewPostgresStorage(db)
	links := []Link{
		{OriginalURL: "https://x.com", ShortURL: "x"},
	}

	mock.ExpectExec("INSERT INTO links \\(url, short_url, user_id\\) VALUES \\(\\$1, \\$2, \\$3\\)").
		WithArgs("https://x.com", "x", nil).
		WillReturnError(&pq.Error{Code: pgerrcode.UniqueViolation})

	err = ps.WriteStorageBatch(links)
	if err == nil {
		t.Fatal("WriteStorageBatch() err = nil, want ErrDuplicateURL")
	}
	if !errors.Is(err, ErrDuplicateURL) {
		t.Errorf("WriteStorageBatch() err = %v, want ErrDuplicateURL", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}
