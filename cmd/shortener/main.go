package main

import (
	"context"
	"crypto/tls"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net"
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
	"github.com/soheilhy/cmux"
	"github.com/zhebrikov/shortener/internal/asyncdelete"
	"github.com/zhebrikov/shortener/internal/audit"
	"github.com/zhebrikov/shortener/internal/auth"
	appconfig "github.com/zhebrikov/shortener/internal/config"
	"github.com/zhebrikov/shortener/internal/db/postgresql"
	"github.com/zhebrikov/shortener/internal/grpcserver"
	"github.com/zhebrikov/shortener/internal/handler"
	"github.com/zhebrikov/shortener/internal/logger"
	"github.com/zhebrikov/shortener/internal/middleware"
	"github.com/zhebrikov/shortener/internal/service"
	"github.com/zhebrikov/shortener/internal/storage"
	"go.uber.org/zap"
	"google.golang.org/grpc"
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
	TLSCertFile   string
	TLSKeyFile    string
	TrustedSubnet string
}

func defaultConfig() Config {
	return Config{
		ServerAddress: defaultServerAddress,
		BaseURL:       defaultBaseURL,
		MigrationPath: defaultMigrationPath,
		TLSCertFile:   defaultTLSCertFile,
		TLSKeyFile:    defaultTLSKeyFile,
	}
}

func configPathFromEnvOrFlag(flagPath string) string {
	if v, ok := os.LookupEnv(envConfigPath); ok {
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
	if file.TrustedSubnet != nil {
		cfg.TrustedSubnet = *file.TrustedSubnet
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
	if visited["tls-cert"] {
		cfg.TLSCertFile = flags.TLSCertFile
	}
	if visited["tls-key"] {
		cfg.TLSKeyFile = flags.TLSKeyFile
	}
	if visited["t"] {
		cfg.TrustedSubnet = flags.TrustedSubnet
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
	tlsCertFile, ok := os.LookupEnv("TLS_CERT")
	if !ok || tlsCertFile == "" {
		tlsCertFile = defaults.TLSCertFile
	}
	tlsKeyFile, ok := os.LookupEnv("TLS_KEY")
	if !ok || tlsKeyFile == "" {
		tlsKeyFile = defaults.TLSKeyFile
	}
	trustedSubnet, ok := os.LookupEnv("TRUSTED_SUBNET")
	if !ok || trustedSubnet == "" {
		trustedSubnet = defaults.TrustedSubnet
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
		TLSCertFile:   tlsCertFile,
		TLSKeyFile:    tlsKeyFile,
		TrustedSubnet: trustedSubnet,
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

// app собирает HTTP- и gRPC-приложение из конфигурации (для main и тестов).
type app struct {
	Handler     http.Handler
	GRPCServer  *grpc.Server
	Addr        string
	Port        string
	Log         *zap.Logger
	Deleter     *asyncdelete.Worker
	DB          *sql.DB
	EnableHTTPS bool
	TLSCertFile string
	TLSKeyFile  string
}

// appDeps группирует зависимости для [newApp] (подменяются в тестах).
type appDeps struct {
	dbConnect  func(dsn string) (*sql.DB, error)
	runMigrate func(migrationsPath, dsn string) error
	loggerNew  func(level string) (*zap.Logger, error)
}

func defaultAppDeps() appDeps {
	md := defaultMigrateDeps()
	return appDeps{
		dbConnect: postgresql.Connection,
		runMigrate: func(migrationsPath, dsn string) error {
			return runMigrations(migrationsPath, dsn, md)
		},
		loggerNew: logger.New,
	}
}

func newApp(cfg Config, ad appDeps) (*app, error) {
	secretKey := cfg.SecretKey
	if secretKey == "" {
		secretKey = "dev-insecure-secret-change-me"
	}

	var store storage.LinkStore
	var db *sql.DB
	if cfg.DatabaseDsn != "" {
		var errConn error
		db, errConn = ad.dbConnect(cfg.DatabaseDsn)
		if errConn != nil {
			return nil, errConn
		}
		migrationsPath := cfg.MigrationPath
		if migrationsPath == "" {
			migrationsPath = "migrations"
		}
		if errMig := ad.runMigrate(migrationsPath, cfg.DatabaseDsn); errMig != nil {
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

	zapLog, err := ad.loggerNew("info")
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
	r.Get("/api/internal/stats", handler.InternalStats(cfg.TrustedSubnet, store))
	r.Post("/api/shorten/batch", h.CreateLinkBatch)
	r.Get("/api/user/urls", h.ListUserURLs)
	r.Delete("/api/user/urls", h.DeleteUserURLs)

	grpcSrv := grpcserver.NewGRPCServer(h.App(), secretKey)

	return &app{
		Handler:     r,
		GRPCServer:  grpcSrv,
		Addr:        cfg.ServerAddress,
		Port:        port,
		Log:         zapLog,
		Deleter:     deleter,
		DB:          db,
		EnableHTTPS: cfg.EnableHTTPS,
		TLSCertFile: cfg.TLSCertFile,
		TLSKeyFile:  cfg.TLSKeyFile,
	}, nil
}

// runDeps группирует зависимости для [run] (подменяются в тестах).
type runDeps struct {
	serve    func(httpSrv *http.Server, grpcSrv *grpc.Server, application *app) error
	shutdown func(ctx context.Context, srv *http.Server, grpcSrv *grpc.Server) error
	logSync  func(*zap.Logger) error
}

func defaultRunDeps() runDeps {
	return runDeps{
		serve:    serveHTTPAndGRPC,
		shutdown: shutdownHTTPAndGRPC,
		logSync: func(l *zap.Logger) error {
			return l.Sync()
		},
	}
}

func serveHTTPAndGRPC(httpSrv *http.Server, grpcSrv *grpc.Server, application *app) error {
	lis, err := net.Listen("tcp", application.Addr)
	if err != nil {
		return err
	}
	if application.EnableHTTPS {
		cert, err := tls.LoadX509KeyPair(application.TLSCertFile, application.TLSKeyFile)
		if err != nil {
			return err
		}
		lis = tls.NewListener(lis, &tls.Config{Certificates: []tls.Certificate{cert}})
	}

	m := cmux.New(lis)
	grpcL := m.MatchWithWriters(cmux.HTTP2MatchHeaderFieldSendSettings("content-type", "application/grpc"))
	httpL := m.Match(cmux.HTTP1Fast())

	errCh := make(chan error, 2)
	go func() {
		if err := grpcSrv.Serve(grpcL); err != nil {
			errCh <- err
		}
	}()
	go func() {
		if err := httpSrv.Serve(httpL); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()
	go func() {
		if err := m.Serve(); err != nil {
			errCh <- err
		}
	}()

	return <-errCh
}

func shutdownHTTPAndGRPC(ctx context.Context, srv *http.Server, grpcSrv *grpc.Server) error {
	stopped := make(chan struct{})
	go func() {
		grpcSrv.GracefulStop()
		close(stopped)
	}()
	if err := srv.Shutdown(ctx); err != nil {
		return err
	}
	select {
	case <-stopped:
	case <-ctx.Done():
		grpcSrv.Stop()
	}
	return nil
}

func run(ctx context.Context, args []string, d runDeps, getConfigFn func(Config) (Config, error), ad appDeps) error {
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
	tlsCertFlag := fs.String("tls-cert", defaultTLSCertFile, "path to TLS certificate file (or TLS_CERT env)")
	tlsKeyFlag := fs.String("tls-key", defaultTLSKeyFile, "path to TLS private key file (or TLS_KEY env)")
	trustedSubnetFlag := fs.String("t", "", "trusted subnet CIDR for internal API (or TRUSTED_SUBNET env)")
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
		TLSCertFile:   *tlsCertFlag,
		TLSKeyFile:    *tlsKeyFlag,
		TrustedSubnet: *trustedSubnetFlag,
	}
	cfg = applyVisitedFlags(cfg, visited, flagCfg)

	cfg, err := getConfigFn(cfg)
	if err != nil {
		return err
	}

	application, err := newApp(cfg, ad)
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
	application.Log.Info("server started", zap.String("address", scheme+"://localhost"+application.Port), zap.Bool("grpc", true))

	srv := &http.Server{Handler: application.Handler}
	serveErr := make(chan error, 1)
	go func() {
		errServe := d.serve(srv, application.GRPCServer, application)
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
		sd = shutdownHTTPAndGRPC
	}
	if err := sd(shutdownCtx, srv, application.GRPCServer); err != nil {
		application.Log.Error("server shutdown", zap.Error(err))
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

// migrateDeps группирует зависимости для [runMigrations] (подменяются в тестах).
type migrateDeps struct {
	newFunc func(sourceURL, databaseURL string) (*migrate.Migrate, error)
	up      func(m *migrate.Migrate) error
	close   func(m *migrate.Migrate)
}

func defaultMigrateDeps() migrateDeps {
	return migrateDeps{
		newFunc: migrate.New,
		up:      defaultMigrateUp,
		close:   func(m *migrate.Migrate) { _, _ = m.Close() },
	}
}

func runMigrations(migrationsPath, dsn string, md migrateDeps) error {
	if abs, err := filepath.Abs(migrationsPath); err == nil {
		migrationsPath = abs
	}
	m, err := md.newFunc("file://"+migrationsPath, dsn)
	if err != nil {
		migrationsPath = "../migrations"
		if abs, err := filepath.Abs(migrationsPath); err == nil {
			migrationsPath = abs
		}
		m, err = md.newFunc("file://"+migrationsPath, dsn)
		if err != nil {
			return err
		}
	}
	defer md.close(m)
	if errUp := md.up(m); errUp != nil && errUp != migrate.ErrNoChange {
		return errUp
	}
	return nil
}

// mainDeps группирует зависимости для [runMain] (подменяются в тестах).
type mainDeps struct {
	exit          func(int)
	notifyContext func() (context.Context, context.CancelFunc)
	runDeps       runDeps
	getConfig     func(Config) (Config, error)
	appDeps       appDeps
}

func defaultMainDeps() mainDeps {
	return mainDeps{
		exit: os.Exit,
		notifyContext: func() (context.Context, context.CancelFunc) {
			return signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
		},
		runDeps:   defaultRunDeps(),
		getConfig: getConfig,
		appDeps:   defaultAppDeps(),
	}
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

func runMain(md mainDeps) {
	printBuildInfo()
	ctx, stop := md.notifyContext()
	defer stop()
	md.exit(exitCode(ctx, os.Args[1:], md.runDeps, md.getConfig, md.appDeps))
}

func main() {
	runMain(defaultMainDeps())
}

func exitCode(ctx context.Context, args []string, d runDeps, getConfigFn func(Config) (Config, error), ad appDeps) int {
	if err := run(ctx, args, d, getConfigFn, ad); err != nil {
		log.Print(err)
		return 1
	}
	return 0
}
