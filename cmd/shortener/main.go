package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/zhebrikov/shortener/internal/asyncdelete"
	"github.com/zhebrikov/shortener/internal/audit"
	"github.com/zhebrikov/shortener/internal/auth"
	appconfig "github.com/zhebrikov/shortener/internal/config"
	"github.com/zhebrikov/shortener/internal/db/postgresql"
	"github.com/zhebrikov/shortener/internal/handler"
	"github.com/zhebrikov/shortener/internal/logger"
	"github.com/zhebrikov/shortener/internal/middleware"
	"github.com/zhebrikov/shortener/internal/service"
	"github.com/zhebrikov/shortener/internal/storage"
	"go.uber.org/zap"
)

var (
	buildVersion = "N/A"
	buildDate    = "N/A"
	buildCommit  = "N/A"
)

const (
	defaultTLSCertFile   = "server.crt"
	defaultTLSKeyFile    = "server.key"
	envConfigPath        = "CONFIG"
	defaultServerAddress = "localhost:8080"
	defaultBaseURL       = "localhost:8080"
	defaultMigrationPath = "migrations"
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
	EnableHTTPS   bool
}

func defaultConfig() Config {
	return Config{
		ServerAddress: defaultServerAddress,
		BaseURL:       defaultBaseURL,
		MigrationPath: defaultMigrationPath,
	}
}

func configPathFromEnvOrFlag(flagPath string) string {
	if v := os.Getenv(envConfigPath); v != "" {
		return v
	}
	return flagPath
}

func applyFileConfig(cfg Config, file appconfig.File) Config {
	if file.ServerAddress != nil {
		cfg.ServerAddress = *file.ServerAddress
	}
	if file.BaseURL != nil {
		cfg.BaseURL = *file.BaseURL
	}
	if file.FileStoragePath != nil {
		cfg.FileStorage = *file.FileStoragePath
	}
	if file.DatabaseDSN != nil {
		cfg.DatabaseDsn = *file.DatabaseDSN
	}
	if file.MigrationsPath != nil {
		cfg.MigrationPath = *file.MigrationsPath
	}
	if file.SecretKey != nil {
		cfg.SecretKey = *file.SecretKey
	}
	if file.AuditFile != nil {
		cfg.AuditFile = *file.AuditFile
	}
	if file.AuditURL != nil {
		cfg.AuditURL = *file.AuditURL
	}
	if file.EnableHTTPS != nil {
		cfg.EnableHTTPS = *file.EnableHTTPS
	}
	return cfg
}

func applyVisitedFlags(cfg Config, visited map[string]bool, flags Config) Config {
	if visited["a"] {
		cfg.ServerAddress = flags.ServerAddress
	}
	if visited["b"] {
		cfg.BaseURL = flags.BaseURL
	}
	if visited["f"] {
		cfg.FileStorage = flags.FileStorage
	}
	if visited["d"] {
		cfg.DatabaseDsn = flags.DatabaseDsn
	}
	if visited["m"] {
		cfg.MigrationPath = flags.MigrationPath
	}
	if visited["k"] {
		cfg.SecretKey = flags.SecretKey
	}
	if visited["audit-file"] {
		cfg.AuditFile = flags.AuditFile
	}
	if visited["audit-url"] {
		cfg.AuditURL = flags.AuditURL
	}
	if visited["s"] {
		cfg.EnableHTTPS = flags.EnableHTTPS
	}
	return cfg
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
	enableHTTPS := defaults.EnableHTTPS
	if v, ok := os.LookupEnv("ENABLE_HTTPS"); ok && v != "" {
		parsed, err := strconv.ParseBool(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid ENABLE_HTTPS: %w", err)
		}
		enableHTTPS = parsed
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
		EnableHTTPS:   enableHTTPS,
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
	Addr    string
	Port    string
	Log     *zap.Logger
	Deleter *asyncdelete.Worker
	DB      *sql.DB
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

	zapLog, err := appLoggerNew("info")
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

	return &app{
		Handler: r,
		Addr:    cfg.ServerAddress,
		Port:    port,
		Log:     zapLog,
		Deleter: deleter,
		DB:      db,
	}, nil
}

// runDeps группирует зависимости для [run] (подменяются в тестах).
type runDeps struct {
	serve    func(srv *http.Server) error
	serveTLS func(srv *http.Server, certFile, keyFile string) error
	shutdown func(ctx context.Context, srv *http.Server) error
	logSync  func(*zap.Logger) error
}

func defaultRunDeps() runDeps {
	return runDeps{
		serve: func(srv *http.Server) error {
			return srv.ListenAndServe()
		},
		serveTLS: func(srv *http.Server, certFile, keyFile string) error {
			return srv.ListenAndServeTLS(certFile, keyFile)
		},
		shutdown: func(ctx context.Context, srv *http.Server) error {
			return srv.Shutdown(ctx)
		},
		logSync: func(l *zap.Logger) error {
			return l.Sync()
		},
	}
}

func run(ctx context.Context, args []string, d runDeps) error {
	fs := flag.NewFlagSet("shortener", flag.ContinueOnError)
	var configPathFlag string
	fs.StringVar(&configPathFlag, "c", "", "path to JSON config file (CONFIG)")
	fs.StringVar(&configPathFlag, "config", "", "path to JSON config file (CONFIG)")
	serverAddrFlag := fs.String("a", defaultServerAddress, "address of the HTTP server")
	baseURLFlag := fs.String("b", defaultBaseURL, "base URL for shortened links")
	fileStorageFlag := fs.String("f", "", "file to store the links (empty = in-memory when no DB)")
	databaseDsnFlag := fs.String("d", "", "database DSN")
	migrationPathFlag := fs.String("m", defaultMigrationPath, "path to the migrations")
	secretKeyFlag := fs.String("k", "", "secret key for signed user cookie (or SECRET_KEY env)")
	auditFileFlag := fs.String("audit-file", "", "append-only audit log file path (or AUDIT_FILE env; empty = disabled)")
	auditURLFlag := fs.String("audit-url", "", "remote audit sink POST URL (or AUDIT_URL env; empty = disabled)")
	enableHTTPSFlag := fs.Bool("s", false, "enable HTTPS (or ENABLE_HTTPS env)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	visited := make(map[string]bool)
	fs.Visit(func(f *flag.Flag) {
		if f.Name != "c" && f.Name != "config" {
			visited[f.Name] = true
		}
	})

	cfg := defaultConfig()
	if configPath := configPathFromEnvOrFlag(configPathFlag); configPath != "" {
		fileCfg, err := appconfig.LoadFile(configPath)
		if err != nil {
			return err
		}
		cfg = applyFileConfig(cfg, fileCfg)
	}

	flagCfg := Config{
		ServerAddress: *serverAddrFlag,
		BaseURL:       *baseURLFlag,
		FileStorage:   *fileStorageFlag,
		DatabaseDsn:   *databaseDsnFlag,
		MigrationPath: *migrationPathFlag,
		SecretKey:     *secretKeyFlag,
		AuditFile:     *auditFileFlag,
		AuditURL:      *auditURLFlag,
		EnableHTTPS:   *enableHTTPSFlag,
	}
	cfg = applyVisitedFlags(cfg, visited, flagCfg)

	cfg, err := appGetConfig(cfg)
	if err != nil {
		return err
	}

	application, err := newApp(cfg)
	if err != nil {
		return err
	}
	defer func() {
		if application.DB != nil {
			_ = application.DB.Close()
		}
	}()

	scheme := "http"
	if cfg.EnableHTTPS {
		scheme = "https"
	}
	application.Log.Info("server started", zap.String("address", scheme+"://localhost"+application.Port))

	srv := &http.Server{Addr: application.Addr, Handler: application.Handler}
	serveErr := make(chan error, 1)
	go func() {
		var errServe error
		if cfg.EnableHTTPS {
			errServe = d.serveTLS(srv, defaultTLSCertFile, defaultTLSKeyFile)
		} else {
			errServe = d.serve(srv)
		}
		if errServe != nil && errServe != http.ErrServerClosed {
			serveErr <- errServe
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
	application.Deleter.Shutdown()
	if syncFn := d.logSync; syncFn != nil {
		_ = syncFn(application.Log)
	}
	return nil
}

func defaultMigrateUp(m *migrate.Migrate) error {
	return m.Up()
}

// osExit, dbConnect и др. подменяются в тестах.
var (
	osExit         = os.Exit
	runDepsFactory = defaultRunDeps
	notifyContext  = func() (context.Context, context.CancelFunc) {
		return signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	}
	dbConnect      = postgresql.Connection
	runMigrate   = runMigrations
	appGetConfig = getConfig
	appLoggerNew = logger.New
	migrateNew   = migrate.New
	migrateUp    = defaultMigrateUp
	migrateClose = func(m *migrate.Migrate) { _, _ = m.Close() }
)

func runMigrations(migrationsPath, dsn string) error {
	if abs, err := filepath.Abs(migrationsPath); err == nil {
		migrationsPath = abs
	}
	m, err := migrateNew("file://"+migrationsPath, dsn)
	if err != nil {
		migrationsPath = "../migrations"
		if abs, err := filepath.Abs(migrationsPath); err == nil {
			migrationsPath = abs
		}
		m, err = migrateNew("file://"+migrationsPath, dsn)
		if err != nil {
			return err
		}
	}
	defer migrateClose(m)
	if errUp := migrateUp(m); errUp != nil && errUp != migrate.ErrNoChange {
		return errUp
	}
	return nil
}

func buildInfoOrNA(s string) string {
	if s == "" {
		return "N/A"
	}
	return s
}

func printBuildInfo() {
	fmt.Println("Build version:", buildInfoOrNA(buildVersion))
	fmt.Println("Build date:", buildInfoOrNA(buildDate))
	fmt.Println("Build commit:", buildInfoOrNA(buildCommit))
}

func main() {
	printBuildInfo()
	ctx, stop := notifyContext()
	defer stop()
	osExit(exitCode(ctx, os.Args[1:]))
}

func exitCode(ctx context.Context, args []string) int {
	if err := run(ctx, args, runDepsFactory()); err != nil {
		log.Print(err)
		return 1
	}
	return 0
}
