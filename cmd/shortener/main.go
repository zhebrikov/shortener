package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/zhebrikov/shortener/internal/handler"
	"github.com/zhebrikov/shortener/internal/logger"
	"github.com/zhebrikov/shortener/internal/middleware"
	"github.com/zhebrikov/shortener/internal/service"
	"github.com/zhebrikov/shortener/internal/storage"
	"go.uber.org/zap"
)

type Config struct {
	ServerAddress string `env:"SERVER_ADDRESS"`
	BaseURL       string `env:"BASE_URL"`
	FileStorage   string `env:"FILE_STORAGE_PATH"`
}

// getConfig возвращает serverAddress и baseURL: приоритет у переменных окружения, иначе флаги.
func getConfig(cfg Config) (Config, error) {
	serverAddress, ok := os.LookupEnv("SERVER_ADDRESS")
	if !ok {
		return Config{}, fmt.Errorf("SERVER_ADDRESS is not set")
	}
	baseURL, ok := os.LookupEnv("BASE_URL")
	if !ok {
		return Config{}, fmt.Errorf("BASE_URL is not set")
	}
	fileStorage, ok := os.LookupEnv("FILE_STORAGE_PATH")
	if !ok {
		return Config{}, fmt.Errorf("FILE_STORAGE_PATH is not set")
	}

	return Config{
		ServerAddress: serverAddress,
		BaseURL:       baseURL,
		FileStorage:   fileStorage,
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
	fileStorageFlag := flag.String("f", "file.json", "file to store the links")
	flag.Parse()

	cfg, err := getConfig(Config{
		ServerAddress: *serverAddrFlag,
		BaseURL:       *baseURLFlag,
		FileStorage:   *fileStorageFlag,
	})
	if err != nil {
		log.Fatal(err)
	}

	storage := storage.NewStorage(cfg.FileStorage)

	port, err := portFromServerAddress(cfg.ServerAddress)
	if err != nil {
		log.Fatal(err)
	}

	zapLog, err := logger.New("info")
	if err != nil {
		log.Fatal(err)
	}

	shortener := service.NewShortener(cfg.BaseURL)
	h := handler.NewShortenerHandler(shortener, storage)

	r := chi.NewRouter()
	r.Use(logger.Middleware(zapLog))
	r.Use(middleware.Gzip)
	r.Post("/", h.CreateLink)
	r.Get("/{shortCode}", h.GetLink)
	r.Post("/api/shorten", h.CreateLinkJSON)

	zapLog.Info("server started", zap.String("address", "http://localhost"+port))

	err = http.ListenAndServe(port, r)
	if err != nil {
		log.Fatal(err)
	}
}
