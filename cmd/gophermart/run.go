package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"

	"github.com/zhebrikov/shortener/internal/gophermart/accrual"
	"github.com/zhebrikov/shortener/internal/gophermart/config"
	"github.com/zhebrikov/shortener/internal/gophermart/handler"
	"github.com/zhebrikov/shortener/internal/gophermart/store"
	"github.com/zhebrikov/shortener/internal/gophermart/worker"
)

// runDeps groups injectable dependencies for [run] (used in tests).
type runDeps struct {
	loadConfig       func() (config.Settings, error)
	openDB           func(driverName, dataSourceName string) (*sql.DB, error)
	ping             func(db *sql.DB) error
	migrateUp        func(migrationPath, databaseURI string) error
	newRouter        func(api *handler.API) http.Handler
	newAccrualClient func(baseURL string, hc *http.Client) *accrual.Client
	pollerInterval   time.Duration
	pollerBatch      int
	serve            func(srv *http.Server) error
	shutdown         func(ctx context.Context, srv *http.Server) error
	stderrSync       func() error
}

func defaultRunDeps() runDeps {
	return runDeps{
		loadConfig: func() (config.Settings, error) {
			return config.Load(true)
		},
		openDB: sql.Open,
		ping: func(db *sql.DB) error {
			return db.Ping()
		},
		migrateUp: migrateUpFile,
		newRouter: func(api *handler.API) http.Handler {
			return handler.NewRouter(api)
		},
		newAccrualClient: accrual.NewClient,
		pollerInterval:   2 * time.Second,
		pollerBatch:      20,
		serve: func(srv *http.Server) error {
			return srv.ListenAndServe()
		},
		shutdown: func(ctx context.Context, srv *http.Server) error {
			return srv.Shutdown(ctx)
		},
		stderrSync: func() error {
			return os.Stderr.Sync()
		},
	}
}

func migrateUpFile(migrationPath, databaseURI string) error {
	mp := migrationPath
	if abs, err := filepath.Abs(migrationPath); err == nil {
		mp = abs
	}
	m, err := migrate.New("file://"+mp, databaseURI)
	if err != nil {
		return err
	}
	defer func() { _, _ = m.Close() }()
	if errUp := m.Up(); errUp != nil && errUp != migrate.ErrNoChange {
		return errUp
	}
	return nil
}

// run starts the HTTP server and accrual worker until ctx is cancelled.
func run(ctx context.Context, d runDeps) error {
	cfg, err := d.loadConfig()
	if err != nil {
		return err
	}

	db, err := d.openDB("postgres", cfg.DatabaseURI)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := d.ping(db); err != nil {
		return err
	}

	if err := d.migrateUp(cfg.MigrationPath, cfg.DatabaseURI); err != nil {
		return err
	}

	st := store.NewPostgres(db)
	api := &handler.API{Store: st, Secret: cfg.JWTSecret}
	r := d.newRouter(api)

	acClient := d.newAccrualClient(cfg.AccrualSystemAddress, nil)
	interval := d.pollerInterval
	if interval <= 0 {
		interval = 2 * time.Second
	}
	batch := d.pollerBatch
	if batch <= 0 {
		batch = 20
	}
	poller := &worker.AccrualPoller{Store: st, Client: acClient, Interval: interval, Batch: batch}
	go poller.Run(ctx)

	srv := &http.Server{Addr: cfg.RunAddress, Handler: r}
	serveErr := make(chan error, 1)
	go func() {
		log.Printf("gophermart listening on %s", cfg.RunAddress)
		err := d.serve(srv)
		if err != nil && err != http.ErrServerClosed {
			serveErr <- err
			return
		}
	}()

	select {
	case <-ctx.Done():
	case err := <-serveErr:
		return err
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	sd := d.shutdown
	if sd == nil {
		sd = func(ctx context.Context, srv *http.Server) error { return srv.Shutdown(ctx) }
	}
	if err := sd(shutdownCtx, srv); err != nil {
		log.Printf("server shutdown: %v", err)
	}
	_ = d.stderrSync()
	return nil
}
