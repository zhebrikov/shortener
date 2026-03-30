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

	"github.com/go-chi/chi/v5"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
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
	return Config{
		ServerAddress: serverAddress,
		BaseURL:       baseURL,
		FileStorage:   fileStorage,
		DatabaseDsn:   databaseDsn,
		MigrationPath: migrationPath,
		SecretKey:     secretKey,
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

func main() {
	serverAddrFlag := flag.String("a", "localhost:8080", "address of the HTTP server")
	baseURLFlag := flag.String("b", "localhost:8080", "base URL for shortened links")
	fileStorageFlag := flag.String("f", "", "file to store the links (empty = in-memory when no DB)")
	databaseDsnFlag := flag.String("d", "", "database DSN")
	migrationPathFlag := flag.String("m", "migrations", "path to the migrations")
	secretKeyFlag := flag.String("k", "", "secret key for signed user cookie (or SECRET_KEY env)")
	flag.Parse()

	cfg, err := getConfig(Config{
		ServerAddress: *serverAddrFlag,
		BaseURL:       *baseURLFlag,
		FileStorage:   *fileStorageFlag,
		DatabaseDsn:   *databaseDsnFlag,
		MigrationPath: *migrationPathFlag,
		SecretKey:     *secretKeyFlag,
	})
	if err != nil {
		log.Fatal(err)
	}
	secretKey := cfg.SecretKey
	if secretKey == "" {
		secretKey = "dev-insecure-secret-change-me"
	}

	// Выбор хранилища: DATABASE_DSN/-d → файл (FILE_STORAGE_PATH/-f) → память.
	var store storage.LinkStore
	var db *sql.DB
	if cfg.DatabaseDsn != "" {
		var errConn error
		db, errConn = postgresql.Connection(cfg.DatabaseDsn)
		if errConn != nil {
			log.Fatal(errConn)
		}
		// Run migrations so user tables exist (required for iteration11 / DB inspect tests).
		migrationsPath := cfg.MigrationPath
		if migrationsPath == "" {
			migrationsPath = "migrations"
		}
		if abs, err := filepath.Abs(migrationsPath); err == nil {
			migrationsPath = abs
		}
		m, errMig := migrate.New("file://"+migrationsPath, cfg.DatabaseDsn)
		if errMig != nil {
			migrationsPath = "../migrations"
			if abs, err := filepath.Abs(migrationsPath); err == nil {
				migrationsPath = abs
			}
			m, errMig = migrate.New("file://"+migrationsPath, cfg.DatabaseDsn)
			if errMig != nil {
				log.Fatal(errMig)
			}
		}
		if errUp := m.Up(); errUp != nil && errUp != migrate.ErrNoChange {
			_, _ = m.Close()
			log.Fatal(errUp)
		}
		_, _ = m.Close()
		store = storage.NewPostgresStorage(db)
	} else if cfg.FileStorage != "" {
		store = storage.NewStorage(cfg.FileStorage)
	} else {
		store = storage.NewMemoryStorage()
	}

	port, err := portFromServerAddress(cfg.ServerAddress)
	if err != nil {
		log.Fatal(err)
	}

	zapLog, err := logger.New("info")
	if err != nil {
		log.Fatal(err)
	}

	shortener := service.NewShortener(cfg.BaseURL)
	h := handler.NewShortenerHandler(shortener, store)

	r := chi.NewRouter()
	r.Use(logger.Middleware(zapLog))
	r.Use(middleware.Gzip)
	r.Use(auth.Middleware(secretKey))
	r.Post("/", h.CreateLink)
	r.Get("/{shortCode}", h.GetLink)
	r.Post("/api/shorten", h.CreateLinkJSON)
	r.Get("/ping", handler.HealthCheck(db))
	r.Post("/api/shorten/batch", h.CreateLinkBatch)
	r.Get("/api/user/urls", h.ListUserURLs)

	zapLog.Info("server started", zap.String("address", "http://localhost"+port))

	err = http.ListenAndServe(port, r)
	if err != nil {
		log.Fatal(err)
	}
}
