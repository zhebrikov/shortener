package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zhebrikov/shortener/internal/service"
	"github.com/zhebrikov/shortener/internal/storage"
)

func benchMemoryStore(links int) (storage.LinkStore, string) {
	store := storage.NewMemoryStorage()
	shortener := service.NewShortener("http://localhost:8080")
	var code string
	for i := 0; i < links; i++ {
		original := fmt.Sprintf("https://example.com/%d", i)
		shortURL, err := shortener.CreateLink(original)
		if err != nil {
			panic(err)
		}
		_ = store.WriteStorage(storage.Link{
			UUID:        i + 1,
			ShortURL:    shortURL,
			OriginalURL: original,
		})
		if i == links/2 {
			code = storage.ShortCodeFromURL(shortURL)
		}
	}
	return store, code
}

func BenchmarkCreateLinkJSON(b *testing.B) {
	shortener := service.NewShortener("http://localhost:8080")
	store := storage.NewMemoryStorage()
	body, _ := json.Marshal(Input{URL: "https://practicum.yandex.ru/new"})

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/shorten", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		CreateLinkJSON(rr, req, shortener, store, nil)
		if rr.Code != http.StatusCreated && rr.Code != http.StatusConflict {
			b.Fatalf("status %d", rr.Code)
		}
	}
}

func BenchmarkGetLink(b *testing.B) {
	shortener := service.NewShortener("http://localhost:8080")
	store, code := benchMemoryStore(5000)
	path := "/" + code

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rr := httptest.NewRecorder()
		GetLink(rr, req, shortener, store, nil)
		if rr.Code != http.StatusTemporaryRedirect {
			b.Fatalf("status %d", rr.Code)
		}
	}
}

func BenchmarkCreateLink_plain(b *testing.B) {
	shortener := service.NewShortener("http://localhost:8080")
	store := storage.NewMemoryStorage()
	body := []byte("https://practicum.yandex.ru/plain")

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
		req.Header.Set("Content-Type", "text/plain")
		rr := httptest.NewRecorder()
		CreateLink(rr, req, shortener, store, nil)
		if rr.Code != http.StatusCreated && rr.Code != http.StatusConflict {
			b.Fatalf("status %d", rr.Code)
		}
	}
}
