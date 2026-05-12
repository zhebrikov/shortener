package main

import (
	"context"
	"database/sql"
	"errors"
	"iter"
	"net"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/zhebrikov/shortener/internal/gophermart/config"
	"github.com/zhebrikov/shortener/internal/gophermart/handler"
	"github.com/zhebrikov/shortener/internal/gophermart/store"
)

type smokeStore struct{}

func (smokeStore) RegisterUser(context.Context, string, string) (string, error) {
	return "", errors.New("noop")
}
func (smokeStore) GetUserByLogin(context.Context, string) (string, string, error) {
	return "", "", sql.ErrNoRows
}
func (smokeStore) UploadOrder(context.Context, string, string) (store.OrderUploadResult, error) {
	return store.UploadAccepted, errors.New("noop")
}
func (smokeStore) ListUserOrders(context.Context, string) iter.Seq2[store.OrderRow, error] {
	return func(yield func(store.OrderRow, error) bool) {}
}
func (smokeStore) Balance(context.Context, string) (float64, float64, error)        { return 0, 0, nil }
func (smokeStore) Withdraw(context.Context, string, string, float64) error          { return nil }
func (smokeStore) ListWithdrawals(context.Context, string) ([]store.WithdrawalRow, error) {
	return nil, nil
}
func (smokeStore) PendingAccrualJobs(context.Context, int) ([]store.AccrualJob, error) {
	return nil, nil
}
func (smokeStore) SyncAccrualStatus(context.Context, int64, string, *float64) error {
	return nil
}

func TestRun_loadConfigError(t *testing.T) {
	d := defaultRunDeps()
	d.loadConfig = func() (config.Settings, error) {
		return config.Settings{}, errors.New("boom")
	}
	err := run(context.Background(), d)
	if err == nil || err.Error() != "boom" {
		t.Fatalf("got %v", err)
	}
}

func TestRun_openDBError(t *testing.T) {
	d := defaultRunDeps()
	d.loadConfig = func() (config.Settings, error) {
		return config.Settings{
			RunAddress:           ":0",
			DatabaseURI:          "postgres://x",
			AccrualSystemAddress: "http://localhost",
			JWTSecret:            "secret-key-32bytes-minimum-length!",
			MigrationPath:        "migrations/gophermart",
		}, nil
	}
	d.openDB = func(string, string) (*sql.DB, error) {
		return nil, errors.New("open failed")
	}
	err := run(context.Background(), d)
	if err == nil || err.Error() != "open failed" {
		t.Fatalf("got %v", err)
	}
}

func TestRun_pingError(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	d := defaultRunDeps()
	d.loadConfig = func() (config.Settings, error) {
		return config.Settings{
			RunAddress:           ":0",
			DatabaseURI:          "postgres://x",
			AccrualSystemAddress: "http://localhost",
			JWTSecret:            "secret-key-32bytes-minimum-length!",
			MigrationPath:        "migrations/gophermart",
		}, nil
	}
	d.openDB = func(string, string) (*sql.DB, error) { return db, nil }
	d.ping = func(*sql.DB) error { return errors.New("ping failed") }

	err = run(context.Background(), d)
	if err == nil || err.Error() != "ping failed" {
		t.Fatalf("got %v", err)
	}
}

func TestRun_migrateUpError(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	d := defaultRunDeps()
	d.loadConfig = func() (config.Settings, error) {
		return config.Settings{
			RunAddress:           ":0",
			DatabaseURI:          "postgres://x",
			AccrualSystemAddress: "http://localhost",
			JWTSecret:            "secret-key-32bytes-minimum-length!",
			MigrationPath:        "migrations/gophermart",
		}, nil
	}
	d.openDB = func(string, string) (*sql.DB, error) { return db, nil }
	d.ping = func(*sql.DB) error { return nil }
	d.migrateUp = func(string, string) error { return errors.New("migrate failed") }

	err = run(context.Background(), d)
	if err == nil || err.Error() != "migrate failed" {
		t.Fatalf("got %v", err)
	}
}

func TestRun_serveError(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	d := defaultRunDeps()
	d.loadConfig = func() (config.Settings, error) {
		return config.Settings{
			RunAddress:           ":0",
			DatabaseURI:          "postgres://x",
			AccrualSystemAddress: "http://localhost",
			JWTSecret:            "secret-key-32bytes-minimum-length!",
			MigrationPath:        "migrations/gophermart",
		}, nil
	}
	d.openDB = func(string, string) (*sql.DB, error) { return db, nil }
	d.ping = func(*sql.DB) error { return nil }
	d.migrateUp = func(string, string) error { return nil }
	d.pollerInterval = time.Hour
	d.serve = func(*http.Server) error { return errors.New("listen failed") }

	err = run(context.Background(), d)
	if err == nil || err.Error() != "listen failed" {
		t.Fatalf("got %v", err)
	}
}

func TestRun_shutdownOK(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	d := defaultRunDeps()
	d.shutdown = nil
	d.loadConfig = func() (config.Settings, error) {
		return config.Settings{
			RunAddress:           ln.Addr().String(),
			DatabaseURI:          "postgres://x",
			AccrualSystemAddress: "http://localhost",
			JWTSecret:            "secret-key-32bytes-minimum-length!",
			MigrationPath:        "migrations/gophermart",
		}, nil
	}
	d.openDB = func(string, string) (*sql.DB, error) { return db, nil }
	d.ping = func(*sql.DB) error { return nil }
	d.migrateUp = func(string, string) error { return nil }
	d.pollerInterval = time.Hour
	d.serve = func(srv *http.Server) error {
		return srv.Serve(ln)
	}
	d.stderrSync = func() error { return nil }

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	if err := run(ctx, d); err != nil {
		t.Fatal(err)
	}
}

func TestRun_shutdownErrorLogged(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	d := defaultRunDeps()
	d.loadConfig = func() (config.Settings, error) {
		return config.Settings{
			RunAddress:           ln.Addr().String(),
			DatabaseURI:          "postgres://x",
			AccrualSystemAddress: "http://localhost",
			JWTSecret:            "secret-key-32bytes-minimum-length!",
			MigrationPath:        "migrations/gophermart",
		}, nil
	}
	d.openDB = func(string, string) (*sql.DB, error) { return db, nil }
	d.ping = func(*sql.DB) error { return nil }
	d.migrateUp = func(string, string) error { return nil }
	d.pollerInterval = time.Hour
	d.serve = func(srv *http.Server) error {
		return srv.Serve(ln)
	}
	d.shutdown = func(context.Context, *http.Server) error {
		return errors.New("shutdown boom")
	}
	d.stderrSync = func() error { return nil }

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	if err := run(ctx, d); err != nil {
		t.Fatal(err)
	}
}

func TestRun_pollerDefaultsApplied(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	d := defaultRunDeps()
	d.loadConfig = func() (config.Settings, error) {
		return config.Settings{
			RunAddress:           ln.Addr().String(),
			DatabaseURI:          "postgres://x",
			AccrualSystemAddress: "http://localhost",
			JWTSecret:            "secret-key-32bytes-minimum-length!",
			MigrationPath:        "migrations/gophermart",
		}, nil
	}
	d.openDB = func(string, string) (*sql.DB, error) { return db, nil }
	d.ping = func(*sql.DB) error { return nil }
	d.migrateUp = func(string, string) error { return nil }
	d.pollerInterval = 0
	d.pollerBatch = 0
	d.serve = func(srv *http.Server) error {
		return srv.Serve(ln)
	}
	d.stderrSync = func() error { return nil }

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	if err := run(ctx, d); err != nil {
		t.Fatal(err)
	}
}

func TestMigrateUpFile_invalidSource(t *testing.T) {
	err := migrateUpFile("/this/path/does/not/exist/migrations", "postgres://localhost/db")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestMigrateUpFile_unreachableDB(t *testing.T) {
	root := filepath.Join("..", "..", "migrations", "gophermart")
	err := migrateUpFile(root, "postgres://127.0.0.1:1/db?sslmode=disable")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDefaultRunDepsPingClosure(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectPing()

	d := defaultRunDeps()
	if err := d.ping(db); err != nil {
		t.Fatal(err)
	}
}

func TestDefaultRunDepsSmoke(t *testing.T) {
	d := defaultRunDeps()
	if d.loadConfig == nil || d.migrateUp == nil || d.newRouter == nil {
		t.Fatal("deps")
	}
	_ = d.newRouter(&handler.API{Store: smokeStore{}, Secret: "secret-key-32bytes-minimum-length!"})
	_ = d.newAccrualClient("http://localhost", nil)
	_ = d.stderrSync()
}
