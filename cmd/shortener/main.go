package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/zhebrikov/shortener/internal/asyncdelete"
	"github.com/zhebrikov/shortener/internal/audit"
	"github.com/zhebrikov/shortener/internal/auth"
	"github.com/zhebrikov/shortener/internal/db/postgresql"
	"github.com/zhebrikov/shortener/internal/handler"
	"github.com/zhebrikov/shortener/internal/logger"
	"github.com/zhebrikov/shortener/internal/middleware"
	"github.com/zhebrikov/shortener/internal/service"
	"github.com/zhebrikov/shortener/internal/storage"
	"go.uber.org/zap"
)

type Config struct {
	ServerAddress string
	BaseURL       string
	FileStorage   string
	DatabaseDsn   string
	MigrationPath string
	SecretKey     string
	AuditFile     string
	AuditURL      string
}

// getConfig возвращает конфиг: переменные окружения имеют приоритет, иначе используются значения по умолчанию (defaults).
func getConfig(defaults Config) (Config, error) {
	serverAddress, ok := os.LookupEnv("SERVER_ADDRESS")
	if !ok || serverAddress == "" {
		serverAddress = defaults.ServerAddress
	}
	baseURL, ok := os.LookupEnv("BASE_URL")
	if !ok || baseURL == "" {
		baseURL = defaults.BaseURL
	}
	fileStorage, ok := os.LookupEnv("FILE_STORAGE_PATH")
	if !ok || fileStorage == "" {
		fileStorage = defaults.FileStorage
	}
	databaseDsn, ok := os.LookupEnv("DATABASE_DSN")
	if !ok || databaseDsn == "" {
		databaseDsn = defaults.DatabaseDsn
	}
	migrationPath, ok := os.LookupEnv("MIGRATIONS_PATH")
	if !ok || migrationPath == "" {
		migrationPath = defaults.MigrationPath
	}
	secretKey, ok := os.LookupEnv("SECRET_KEY")
	if !ok || secretKey == "" {
		secretKey = defaults.SecretKey
	}
	auditFile, ok := os.LookupEnv("AUDIT_FILE")
	if !ok || auditFile == "" {
		auditFile = defaults.AuditFile
	}
	auditURL, ok := os.LookupEnv("AUDIT_URL")
	if !ok || auditURL == "" {
		auditURL = defaults.AuditURL
	}
	return Config{
		ServerAddress: serverAddress,
		BaseURL:       baseURL,
		FileStorage:   fileStorage,
		DatabaseDsn:   databaseDsn,
		MigrationPath: migrationPath,
		SecretKey:     secretKey,
		AuditFile:     auditFile,
		AuditURL:      auditURL,
	}, nil
}

// portFromServerAddress возвращает порт из адреса вида "host:port" или ":port".
func portFromServerAddress(serverAddress string) (string, error) {
	idx := strings.Index(serverAddress, ":")
	if idx == -1 {
		return "", fmt.Errorf("server address must contain port")
	}
	return serverAddress[idx:], nil
}

// app собирает HTTP-приложение из конфигурации (для main и тестов).
type app struct {
	Handler http.Handler
	Port    string
	Log     *zap.Logger
}

func newApp(cfg Config) (*app, error) {
	secretKey := cfg.SecretKey
	if secretKey == "" {
		secretKey = "dev-insecure-secret-change-me"
	}

	var store storage.LinkStore
	var db *sql.DB
	if cfg.DatabaseDsn != "" {
		var errConn error
		db, errConn = dbConnect(cfg.DatabaseDsn)
		if errConn != nil {
			return nil, errConn
		}
		migrationsPath := cfg.MigrationPath
		if migrationsPath == "" {
			migrationsPath = "migrations"
		}
		if errMig := runMigrate(migrationsPath, cfg.DatabaseDsn); errMig != nil {
			return nil, errMig
		}
		store = storage.NewPostgresStorage(db)
	} else if cfg.FileStorage != "" {
		store = storage.NewStorage(cfg.FileStorage)
	} else {
		store = storage.NewMemoryStorage()
	}

	port, err := portFromServerAddress(cfg.ServerAddress)
	if err != nil {
		return nil, err
	}

	zapLog, err := logger.New("info")
	if err != nil {
		return nil, err
	}

	shortener := service.NewShortener(cfg.BaseURL)
	deleter := asyncdelete.NewWorker(store)

	var auditObservers []audit.Observer
	if cfg.AuditFile != "" {
		auditObservers = append(auditObservers, audit.NewFileObserver(cfg.AuditFile))
	}
	if cfg.AuditURL != "" {
		auditObservers = append(auditObservers, audit.NewHTTPObserver(cfg.AuditURL, &http.Client{Timeout: 5 * time.Second}))
	}
	auditPub := audit.NewPublisher(auditObservers...)

	h := handler.NewShortenerHandler(shortener, store, deleter, auditPub)

	r := chi.NewRouter()
	r.Use(logger.Middleware(zapLog))
	r.Use(middleware.Gzip)
	r.Use(auth.Middleware(secretKey))
	r.Post("/", h.CreateLink)
	r.Get("/{shortCode}", h.GetLink)
	r.Post("/api/shorten", h.CreateLinkJSON)
	r.Get("/ping", handler.HealthCheck(db))
	r.Post("/api/shorten/batch", h.CreateLinkBatch)
	r.Get("/api/user/urls", func(w http.ResponseWriter, r *http.Request) {
		handler.ListUserURLs(w, r, store)
	})
	r.Delete("/api/user/urls", h.DeleteUserURLs)

	return &app{Handler: r, Port: port, Log: zapLog}, nil
}

func run(args []string, listen func(string, http.Handler) error) error {
	fs := flag.NewFlagSet("shortener", flag.ContinueOnError)
	serverAddrFlag := fs.String("a", "localhost:8080", "address of the HTTP server")
	baseURLFlag := fs.String("b", "localhost:8080", "base URL for shortened links")
	fileStorageFlag := fs.String("f", "", "file to store the links (empty = in-memory when no DB)")
	databaseDsnFlag := fs.String("d", "", "database DSN")
	migrationPathFlag := fs.String("m", "migrations", "path to the migrations")
	secretKeyFlag := fs.String("k", "", "secret key for signed user cookie (or SECRET_KEY env)")
	auditFileFlag := fs.String("audit-file", "", "append-only audit log file path (or AUDIT_FILE env; empty = disabled)")
	auditURLFlag := fs.String("audit-url", "", "remote audit sink POST URL (or AUDIT_URL env; empty = disabled)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := getConfig(Config{
		ServerAddress: *serverAddrFlag,
		BaseURL:       *baseURLFlag,
		FileStorage:   *fileStorageFlag,
		DatabaseDsn:   *databaseDsnFlag,
		MigrationPath: *migrationPathFlag,
		SecretKey:     *secretKeyFlag,
		AuditFile:     *auditFileFlag,
		AuditURL:      *auditURLFlag,
	})
	if err != nil {
		return err
	}

	application, err := newApp(cfg)
	if err != nil {
		return err
	}

	application.Log.Info("server started", zap.String("address", "http://localhost"+application.Port))
	return listen(application.Port, application.Handler)
}

// osExit, appListen и dbConnect подменяются в тестах.
var (
	osExit     = os.Exit
	appListen  = http.ListenAndServe
	dbConnect  = postgresql.Connection
	runMigrate = runMigrations
)

func runMigrations(migrationsPath, dsn string) error {
	if abs, err := filepath.Abs(migrationsPath); err == nil {
		migrationsPath = abs
	}
	m, err := migrate.New("file://"+migrationsPath, dsn)
	if err != nil {
		migrationsPath = "../migrations"
		if abs, err := filepath.Abs(migrationsPath); err == nil {
			migrationsPath = abs
		}
		m, err = migrate.New("file://"+migrationsPath, dsn)
		if err != nil {
			return err
		}
	}
	defer func() { _, _ = m.Close() }()
	if errUp := m.Up(); errUp != nil && errUp != migrate.ErrNoChange {
		return errUp
	}
	return nil
}

func main() {
	osExit(exitCode(os.Args[1:], appListen))
}

func exitCode(args []string, listen func(string, http.Handler) error) int {
	if err := run(args, listen); err != nil {
		log.Print(err)
		return 1
	}
	return 0
}
