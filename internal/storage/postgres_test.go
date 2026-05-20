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

	mock.ExpectExec("INSERT INTO links \\(url, short_url, user_id, is_deleted\\) VALUES \\(\\$1, \\$2, \\$3, \\$4\\)").
		WithArgs(link.OriginalURL, link.ShortURL, nil, false).
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

	mock.ExpectExec("INSERT INTO links \\(url, short_url, user_id, is_deleted\\) VALUES \\(\\$1, \\$2, \\$3, \\$4\\)").
		WithArgs(link.OriginalURL, link.ShortURL, nil, false).
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

	mock.ExpectExec("INSERT INTO links \\(url, short_url, user_id, is_deleted\\) VALUES \\(\\$1, \\$2, \\$3, \\$4\\)").
		WithArgs(link.OriginalURL, link.ShortURL, nil, false).
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
	rows := sqlmock.NewRows([]string{"url", "short_url", "user_id", "is_deleted"})
	mock.ExpectQuery("SELECT url, short_url, user_id, is_deleted FROM links ORDER BY id").
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

func TestPostgresStorage_ReadStorage_WithUserID(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ps := NewPostgresStorage(db)
	const uid = "550e8400-e29b-41d4-a716-446655440000"
	rows := sqlmock.NewRows([]string{"url", "short_url", "user_id", "is_deleted"}).
		AddRow("https://a.com", "s1", uid, true)
	mock.ExpectQuery("SELECT url, short_url, user_id, is_deleted FROM links ORDER BY id").
		WillReturnRows(rows)

	links, err := ps.ReadStorage()
	if err != nil || len(links) != 1 || links[0].UserID != uid || !links[0].IsDeleted {
		t.Fatalf("links = %+v, err = %v", links, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestPostgresStorage_ReadStorage_WithRows(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() err = %v", err)
	}
	defer db.Close()

	ps := NewPostgresStorage(db)
	rows := sqlmock.NewRows([]string{"url", "short_url", "user_id", "is_deleted"}).
		AddRow("https://first.com", "f1", nil, false).
		AddRow("https://second.com", "s2", nil, false)
	mock.ExpectQuery("SELECT url, short_url, user_id, is_deleted FROM links ORDER BY id").
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
	mock.ExpectQuery("SELECT url, short_url, user_id, is_deleted FROM links ORDER BY id").
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
	rows := sqlmock.NewRows([]string{"url", "short_url", "user_id", "is_deleted"}).
		AddRow("https://example.com/page", "abc123", nil, false)
	mock.ExpectQuery("SELECT url, short_url, user_id, is_deleted FROM links WHERE short_url = \\$1").
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
	mock.ExpectQuery("SELECT url, short_url, user_id, is_deleted FROM links WHERE short_url = \\$1").
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
	mock.ExpectQuery("SELECT url, short_url, user_id, is_deleted FROM links WHERE short_url = \\$1").
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

	mock.ExpectExec("INSERT INTO links \\(url, short_url, user_id, is_deleted\\) VALUES \\(\\$1, \\$2, \\$3, \\$4\\),\\(\\$5, \\$6, \\$7, \\$8\\)").
		WithArgs("https://a.com", "s1", nil, false, "https://b.com", "s2", nil, false).
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

	mock.ExpectExec("INSERT INTO links \\(url, short_url, user_id, is_deleted\\) VALUES \\(\\$1, \\$2, \\$3, \\$4\\)").
		WithArgs("https://x.com", "x", nil, false).
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

func TestPostgresStorage_SoftDeleteURLsByUser(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() err = %v", err)
	}
	defer db.Close()

	ps := NewPostgresStorage(db)
	const uid = "550e8400-e29b-41d4-a716-446655440000"
	codes := []string{"x1", "y2"}

	mock.ExpectExec("UPDATE links SET is_deleted = true").
		WithArgs(uid, pq.Array(codes)).
		WillReturnResult(sqlmock.NewResult(0, 2))

	if err := ps.SoftDeleteURLsByUser(uid, codes); err != nil {
		t.Fatalf("SoftDeleteURLsByUser() err = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestPostgresStorage_GetLinksByUserID(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ps := NewPostgresStorage(db)
	const uid = "550e8400-e29b-41d4-a716-446655440000"
	if links, err := ps.GetLinksByUserID(""); err != nil || links != nil {
		t.Fatalf("empty user: %+v, %v", links, err)
	}

	rows := sqlmock.NewRows([]string{"url", "short_url"}).
		AddRow("https://a.com", "http://localhost/a1")
	mock.ExpectQuery("SELECT url, short_url FROM links WHERE user_id").
		WithArgs(uid).
		WillReturnRows(rows)

	links, err := ps.GetLinksByUserID(uid)
	if err != nil || len(links) != 1 || links[0].OriginalURL != "https://a.com" {
		t.Fatalf("got %+v, %v", links, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestPostgresStorage_GetLinkByShortCode(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ps := NewPostgresStorage(db)
	if _, err := ps.GetLinkByShortCode(""); !errors.Is(err, ErrLinkNotFound) {
		t.Fatalf("empty code: %v", err)
	}

	rows := sqlmock.NewRows([]string{"url", "short_url", "user_id", "is_deleted"}).
		AddRow("https://a.com", "http://localhost/c1", nil, false)
	mock.ExpectQuery("SELECT url, short_url, user_id, is_deleted FROM links").
		WithArgs("c1").
		WillReturnRows(rows)

	link, err := ps.GetLinkByShortCode("c1")
	if err != nil || link.OriginalURL != "https://a.com" {
		t.Fatalf("got %+v, %v", link, err)
	}

	mock.ExpectQuery("SELECT url, short_url, user_id, is_deleted FROM links").
		WithArgs("missing").
		WillReturnError(sql.ErrNoRows)
	if _, err := ps.GetLinkByShortCode("missing"); !errors.Is(err, ErrLinkNotFound) {
		t.Fatalf("not found: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestPostgresStorage_GetLinkByShortCode_withUserID(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ps := NewPostgresStorage(db)
	const uid = "550e8400-e29b-41d4-a716-446655440000"
	rows := sqlmock.NewRows([]string{"url", "short_url", "user_id", "is_deleted"}).
		AddRow("https://a.com", "http://localhost/c1", uid, false)
	mock.ExpectQuery("SELECT url, short_url, user_id, is_deleted FROM links").
		WithArgs("c1").
		WillReturnRows(rows)

	link, err := ps.GetLinkByShortCode("c1")
	if err != nil || link.UserID != uid {
		t.Fatalf("link = %+v, err = %v", link, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestPostgresStorage_GetShortURLByOriginalURL_notFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ps := NewPostgresStorage(db)
	mock.ExpectQuery("SELECT short_url FROM links WHERE url").
		WithArgs("https://missing.com").
		WillReturnError(sql.ErrNoRows)

	if _, err := ps.GetShortURLByOriginalURL("https://missing.com"); err == nil {
		t.Fatal("expected error")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestPostgresStorage_SoftDeleteURLsByUser_noop(t *testing.T) {
	ps := NewPostgresStorage(nil)
	if err := ps.SoftDeleteURLsByUser("", []string{"x"}); err != nil {
		t.Fatal(err)
	}
}

func TestPostgresStorage_ReadStorage_secondRowScanError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ps := NewPostgresStorage(db)
	rows := sqlmock.NewRows([]string{"url", "short_url", "user_id", "is_deleted"}).
		AddRow("https://a.com", "s1", nil, false).
		AddRow("https://b.com", "s2", nil, false).
		RowError(1, sql.ErrConnDone)
	mock.ExpectQuery("SELECT url, short_url, user_id, is_deleted FROM links ORDER BY id").
		WillReturnRows(rows)

	if _, err := ps.ReadStorage(); err != sql.ErrConnDone {
		t.Fatalf("err = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestPostgresStorage_ReadStorage_emptyRowsErr(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ps := NewPostgresStorage(db)
	rows := sqlmock.NewRows([]string{"url", "short_url", "user_id", "is_deleted"}).
		CloseError(sql.ErrConnDone)
	mock.ExpectQuery("SELECT url, short_url, user_id, is_deleted FROM links ORDER BY id").
		WillReturnRows(rows)

	if _, err := ps.ReadStorage(); err != sql.ErrConnDone {
		t.Fatalf("err = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestPostgresStorage_ReadStorage_rowsErr(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ps := NewPostgresStorage(db)
	rows := sqlmock.NewRows([]string{"url", "short_url", "user_id", "is_deleted"}).
		AddRow("https://a.com", "s1", nil, false).
		CloseError(sql.ErrConnDone)
	mock.ExpectQuery("SELECT url, short_url, user_id, is_deleted FROM links ORDER BY id").
		WillReturnRows(rows)

	if _, err := ps.ReadStorage(); err != sql.ErrConnDone {
		t.Fatalf("err = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestPostgresStorage_GetByShortURL_withUserID(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ps := NewPostgresStorage(db)
	const uid = "550e8400-e29b-41d4-a716-446655440000"
	rows := sqlmock.NewRows([]string{"url", "short_url", "user_id", "is_deleted"}).
		AddRow("https://a.com", "s1", uid, false)
	mock.ExpectQuery("SELECT url, short_url, user_id, is_deleted FROM links WHERE short_url = \\$1").
		WithArgs("s1").
		WillReturnRows(rows)

	link, err := ps.GetByShortURL("s1")
	if err != nil || link == nil || link.UserID != uid {
		t.Fatalf("link = %+v, err = %v", link, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestPostgresStorage_GetLinksByUserID_secondRowScanError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ps := NewPostgresStorage(db)
	const uid = "550e8400-e29b-41d4-a716-446655440000"
	rows := sqlmock.NewRows([]string{"url", "short_url"}).
		AddRow("https://a.com", "s1").
		AddRow("https://b.com", "s2").
		RowError(1, sql.ErrConnDone)
	mock.ExpectQuery("SELECT url, short_url FROM links WHERE user_id").
		WithArgs(uid).
		WillReturnRows(rows)

	if _, err := ps.GetLinksByUserID(uid); err != sql.ErrConnDone {
		t.Fatalf("err = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestPostgresStorage_GetLinksByUserID_emptyRowsErr(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ps := NewPostgresStorage(db)
	const uid = "550e8400-e29b-41d4-a716-446655440000"
	rows := sqlmock.NewRows([]string{"url", "short_url"}).CloseError(sql.ErrConnDone)
	mock.ExpectQuery("SELECT url, short_url FROM links WHERE user_id").
		WithArgs(uid).
		WillReturnRows(rows)

	if _, err := ps.GetLinksByUserID(uid); err != sql.ErrConnDone {
		t.Fatalf("err = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestPostgresStorage_GetLinksByUserID_rowsErr(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ps := NewPostgresStorage(db)
	const uid = "550e8400-e29b-41d4-a716-446655440000"
	rows := sqlmock.NewRows([]string{"url", "short_url"}).
		AddRow("https://a.com", "s1").
		CloseError(sql.ErrConnDone)
	mock.ExpectQuery("SELECT url, short_url FROM links WHERE user_id").
		WithArgs(uid).
		WillReturnRows(rows)

	if _, err := ps.GetLinksByUserID(uid); err != sql.ErrConnDone {
		t.Fatalf("err = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestPostgresStorage_GetLinksByUserID_scanError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ps := NewPostgresStorage(db)
	const uid = "550e8400-e29b-41d4-a716-446655440000"
	rows := sqlmock.NewRows([]string{"url", "short_url"}).
		AddRow("https://a.com", "s1").
		RowError(0, sql.ErrConnDone)
	mock.ExpectQuery("SELECT url, short_url FROM links WHERE user_id").
		WithArgs(uid).
		WillReturnRows(rows)

	if _, err := ps.GetLinksByUserID(uid); err != sql.ErrConnDone {
		t.Fatalf("err = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestPostgresStorage_ReadStorage_scanError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ps := NewPostgresStorage(db)
	rows := sqlmock.NewRows([]string{"url", "short_url", "user_id", "is_deleted"}).
		AddRow("https://a.com", "s1", nil, false).
		RowError(0, sql.ErrConnDone)
	mock.ExpectQuery("SELECT url, short_url, user_id, is_deleted FROM links ORDER BY id").
		WillReturnRows(rows)

	if _, err := ps.ReadStorage(); err != sql.ErrConnDone {
		t.Fatalf("err = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestPostgresStorage_WriteStorage_withUserID(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ps := NewPostgresStorage(db)
	const uid = "550e8400-e29b-41d4-a716-446655440000"
	link := Link{ShortURL: "s1", OriginalURL: "https://a.com", UserID: uid}

	mock.ExpectExec("INSERT INTO links \\(url, short_url, user_id, is_deleted\\) VALUES \\(\\$1, \\$2, \\$3, \\$4\\)").
		WithArgs(link.OriginalURL, link.ShortURL, uid, false).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := ps.WriteStorage(link); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestPostgresStorage_WriteStorageBatch_execError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ps := NewPostgresStorage(db)
	links := []Link{{OriginalURL: "https://a.com", ShortURL: "s1", UserID: "550e8400-e29b-41d4-a716-446655440000"}}

	mock.ExpectExec("INSERT INTO links").
		WillReturnError(sql.ErrConnDone)

	if err := ps.WriteStorageBatch(links); err != sql.ErrConnDone {
		t.Fatalf("err = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestPostgresStorage_GetShortURLByOriginalURL_queryError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ps := NewPostgresStorage(db)
	mock.ExpectQuery("SELECT short_url FROM links WHERE url").
		WithArgs("https://a.com").
		WillReturnError(sql.ErrConnDone)

	if _, err := ps.GetShortURLByOriginalURL("https://a.com"); err != sql.ErrConnDone {
		t.Fatalf("err = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestPostgresStorage_GetLinksByUserID_queryError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ps := NewPostgresStorage(db)
	const uid = "550e8400-e29b-41d4-a716-446655440000"
	mock.ExpectQuery("SELECT url, short_url FROM links WHERE user_id").
		WithArgs(uid).
		WillReturnError(sql.ErrConnDone)

	if _, err := ps.GetLinksByUserID(uid); err != sql.ErrConnDone {
		t.Fatalf("err = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestPostgresStorage_GetLinkByShortCode_queryError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ps := NewPostgresStorage(db)
	mock.ExpectQuery("SELECT url, short_url, user_id, is_deleted FROM links").
		WithArgs("c1").
		WillReturnError(sql.ErrConnDone)

	if _, err := ps.GetLinkByShortCode("c1"); err != sql.ErrConnDone {
		t.Fatalf("err = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestPostgresStorage_SoftDeleteURLsByUser_error(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ps := NewPostgresStorage(db)
	const uid = "550e8400-e29b-41d4-a716-446655440000"
	mock.ExpectExec("UPDATE links SET is_deleted = true").
		WithArgs(uid, pq.Array([]string{"x"})).
		WillReturnError(sql.ErrConnDone)

	if err := ps.SoftDeleteURLsByUser(uid, []string{"x"}); err != sql.ErrConnDone {
		t.Fatalf("err = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestPostgresStorage_NextLinkUUID_error(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ps := NewPostgresStorage(db)
	mock.ExpectQuery("SELECT COALESCE\\(MAX\\(id\\), 0\\) \\+ 1 FROM links").
		WillReturnError(sql.ErrConnDone)

	n, err := ps.NextLinkUUID()
	if err != sql.ErrConnDone || n != 1 {
		t.Fatalf("got (%d, %v)", n, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestPostgresStorage_NextLinkUUID(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ps := NewPostgresStorage(db)
	mock.ExpectQuery("SELECT COALESCE\\(MAX\\(id\\), 0\\) \\+ 1 FROM links").
		WillReturnRows(sqlmock.NewRows([]string{"n"}).AddRow(42))

	n, err := ps.NextLinkUUID()
	if err != nil || n != 42 {
		t.Fatalf("got %d, %v", n, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
