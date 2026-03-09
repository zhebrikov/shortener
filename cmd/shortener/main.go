package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/go-chi/chi/v5"
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

	return Config{
		ServerAddress: serverAddress,
		BaseURL:       baseURL,
		FileStorage:   fileStorage,
		DatabaseDsn:   databaseDsn,
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
	flag.Parse()

	cfg, err := getConfig(Config{
		ServerAddress: *serverAddrFlag,
		BaseURL:       *baseURLFlag,
		FileStorage:   *fileStorageFlag,
		DatabaseDsn:   *databaseDsnFlag,
	})
	if err != nil {
		log.Fatal(err)
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
	r.Post("/", h.CreateLink)
	r.Get("/{shortCode}", h.GetLink)
	r.Post("/api/shorten", h.CreateLinkJSON)
	r.Get("/ping", handler.HealthCheck(db))
	r.Post("/api/shorten/batch", h.CreateLinkBatch)

	zapLog.Info("server started", zap.String("address", "http://localhost"+port))

	err = http.ListenAndServe(port, r)
	if err != nil {
		log.Fatal(err)
	}
}
