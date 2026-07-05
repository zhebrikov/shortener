package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/zhebrikov/shortener/internal/asyncdelete"
	"github.com/zhebrikov/shortener/internal/auth"
	"github.com/zhebrikov/shortener/internal/handler"
	"github.com/zhebrikov/shortener/internal/service"
	"github.com/zhebrikov/shortener/internal/storage"
)

func exampleHandler(shortener *service.Shortener, store storage.LinkStore) *handler.ShortenerHandler {
	return handler.NewShortenerHandler(shortener, store, asyncdelete.NewWorker(store), nil)
}

// Example демонстрирует типичный сценарий: создать короткую ссылку и выполнить редирект.
func Example() {
	shortener := service.NewShortener("localhost:8080")
	store := storage.NewMemoryStorage()
	h := exampleHandler(shortener, store)

	// POST / — сокращение URL (text/plain).
	createReq := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("https://example.com/page"))
	createReq.Header.Set("Content-Type", "text/plain")
	createRR := httptest.NewRecorder()
	handler.CreateLink(createRR, createReq, h)
	fmt.Println(createRR.Code)

	shortURL := strings.TrimSpace(createRR.Body.String())
	shortCode := storage.ShortCodeFromURL(shortURL)

	// GET /{shortCode} — редирект на оригинал.
	getReq := httptest.NewRequest(http.MethodGet, "/"+shortCode, nil)
	getRR := httptest.NewRecorder()
	handler.GetLink(getRR, getReq, h)
	fmt.Println(getRR.Code)
	fmt.Println(getRR.Header().Get("Location"))

	// Output:
	// 201
	// 307
	// https://example.com/page
}

// ExampleNewShortenerHandler показывает сборку обработчика с зависимостями сервиса.
func ExampleNewShortenerHandler() {
	shortener := service.NewShortener("localhost:8080")
	store := storage.NewMemoryStorage()
	h := handler.NewShortenerHandler(shortener, store, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("https://example.com/page"))
	req.Header.Set("Content-Type", "text/plain")
	rr := httptest.NewRecorder()
	h.CreateLink(rr, req)

	fmt.Println(rr.Code)
	// Output:
	// 201
}

// ExampleCreateLink — POST / с телом text/plain.
func ExampleCreateLink() {
	shortener := service.NewShortener("localhost:8080")
	store := storage.NewMemoryStorage()
	h := exampleHandler(shortener, store)

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("https://example.com/page"))
	req.Header.Set("Content-Type", "text/plain")
	rr := httptest.NewRecorder()

	handler.CreateLink(rr, req, h)
	fmt.Println(rr.Code)

	// Output:
	// 201
}

// ExampleGetLink — GET /{shortCode}, ответ 307 Temporary Redirect.
func ExampleGetLink() {
	shortener := service.NewShortener("localhost:8080")
	store := storage.NewMemoryStorage()

	originalURL := "https://example.com/target"
	shortURL, _ := shortener.CreateLink(originalURL)
	_ = store.WriteStorage(storage.Link{
		UUID:        1,
		ShortURL:    shortURL,
		OriginalURL: originalURL,
	})

	shortCode := storage.ShortCodeFromURL(shortURL)
	req := httptest.NewRequest(http.MethodGet, "/"+shortCode, nil)
	rr := httptest.NewRecorder()

	handler.GetLink(rr, req, exampleHandler(shortener, store))
	fmt.Println(rr.Code)
	fmt.Println(rr.Header().Get("Location"))

	// Output:
	// 307
	// https://example.com/target
}

// ExampleCreateLinkJSON — POST /api/shorten с JSON-телом.
func ExampleCreateLinkJSON() {
	shortener := service.NewShortener("localhost:8080")
	store := storage.NewMemoryStorage()
	h := exampleHandler(shortener, store)

	body, _ := json.Marshal(handler.Input{URL: "https://example.com/page"})
	req := httptest.NewRequest(http.MethodPost, "/api/shorten", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	handler.CreateLinkJSON(rr, req, h)
	fmt.Println(rr.Code)

	// Output:
	// 201
}

// ExampleCreateLinkBatch — POST /api/shorten/batch, пакетное сокращение.
func ExampleCreateLinkBatch() {
	shortener := service.NewShortener("localhost:8080")
	store := storage.NewMemoryStorage()

	batch := []handler.BatchRequestItem{
		{CorrelationID: "req-1", OriginalURL: "https://example.com/a"},
		{CorrelationID: "req-2", OriginalURL: "https://example.com/b"},
	}
	body, _ := json.Marshal(batch)
	req := httptest.NewRequest(http.MethodPost, "/api/shorten/batch", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	handler.CreateLinkBatch(rr, req, shortener, store)
	fmt.Println(rr.Code)

	// Output:
	// 201
}

// ExampleDeleteUserURLs — DELETE /api/user/urls, асинхронное удаление (202).
func ExampleDeleteUserURLs() {
	body, _ := json.Marshal([]string{"abc12345"})
	req := httptest.NewRequest(http.MethodDelete, "/api/user/urls", bytes.NewReader(body))
	ctx := auth.WithUserID(context.Background(), "user-1")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	// Worker может быть nil: хендлер всё равно отвечает 202 Accepted.
	handler.DeleteUserURLs(rr, req, nil)
	fmt.Println(rr.Code)

	// Output:
	// 202
}

// ExampleListUserURLs — GET /api/user/urls, список ссылок пользователя.
func ExampleListUserURLs() {
	store := storage.NewMemoryStorage()
	_ = store.WriteStorage(storage.Link{
		UUID:        1,
		ShortURL:    "http://localhost:8080/abc",
		OriginalURL: "https://example.com/mine",
		UserID:      "user-1",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/user/urls", nil)
	ctx := auth.WithUserID(context.Background(), "user-1")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	handler.ListUserURLs(rr, req, exampleHandler(service.NewShortener("localhost:8080"), store))
	fmt.Println(rr.Code)

	// Output:
	// 200
}

// ExampleHealthCheck — GET /ping, проверка доступности БД (nil — всегда 200).
func ExampleHealthCheck() {
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	rr := httptest.NewRecorder()

	handler.HealthCheck(nil)(rr, req)
	fmt.Println(rr.Code)

	// Output:
	// 200
}
