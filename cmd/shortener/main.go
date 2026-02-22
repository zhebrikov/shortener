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

// getConfig возвращает serverAddress и baseURL: приоритет у переменных окружения, иначе флаги.
func getConfig(serverAddrFlag, baseURLFlag, fileStorageFlag string) (serverAddress, baseURL, fileStorage string) {
	serverAddress = os.Getenv("SERVER_ADDRESS")
	if serverAddress == "" {
		serverAddress = serverAddrFlag
	}
	baseURL = os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = baseURLFlag
	}
	fileStorage = os.Getenv("FILE_STORAGE_PATH")
	if fileStorage == "" {
		fileStorage = fileStorageFlag
	}
	return serverAddress, baseURL, fileStorage
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

	serverAddress, baseURL, fileStorage := getConfig(*serverAddrFlag, *baseURLFlag, *fileStorageFlag)

	storage := storage.NewStorage(fileStorage)

	port, err := portFromServerAddress(serverAddress)
	if err != nil {
		log.Fatal(err)
	}

	if err := logger.Initialize("info"); err != nil {
		log.Fatal(err)
	}

	shortener := service.NewShortener(baseURL)
	h := handler.NewShortenerHandler(shortener, storage)

	r := chi.NewRouter()
	r.Use(logger.Middleware)
	r.Use(middleware.Gzip)
	r.Post("/", h.CreateLink)
	r.Get("/{shortCode}", h.GetLink)
	r.Post("/api/shorten", h.CreateLinkJson)

	logger.Log.Info("server started", zap.String("address", "http://localhost"+port))

	err = http.ListenAndServe(port, r)
	if err != nil {
		log.Fatal(err)
	}
}
