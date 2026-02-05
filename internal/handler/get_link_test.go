package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zhebrikov/shortener/internal/service"
)

func TestGetLink_RedirectWhenFound(t *testing.T) {
	shortener := service.NewShortener("localhost:8080")

	// Создаём ссылку через сервис напрямую
	originalURL := "https://example.com/redirect-target"
	shortURL := shortener.CreateLink(originalURL)
	shortCode := shortURL[strings.LastIndex(shortURL, "/")+1:]

	req := httptest.NewRequest(http.MethodGet, "/"+shortCode, nil)
	rr := httptest.NewRecorder()
	GetLink(rr, req, shortener)

	if rr.Code != http.StatusTemporaryRedirect {
		t.Errorf("GetLink: got status %d, want %d", rr.Code, http.StatusTemporaryRedirect)
	}
	if loc := rr.Header().Get("Location"); loc != originalURL {
		t.Errorf("GetLink: Location = %q, want %q", loc, originalURL)
	}
}

func TestGetLink_NotFound(t *testing.T) {
	shortener := service.NewShortener("localhost:8080")

	req := httptest.NewRequest(http.MethodGet, "/nonexistent123", nil)
	rr := httptest.NewRecorder()
	GetLink(rr, req, shortener)

	if rr.Code != http.StatusNotFound {
		t.Errorf("GetLink: got status %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestGetLink_RootPath_NotFound(t *testing.T) {
	shortener := service.NewShortener("localhost:8080")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	GetLink(rr, req, shortener)

	if rr.Code != http.StatusNotFound {
		t.Errorf("GetLink GET /: got status %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestGetLink_MethodNotAllowed(t *testing.T) {
	shortener := service.NewShortener("localhost:8080")
	shortener.CreateLink("https://example.com")
	shortURL := shortener.CreateLink("https://example.com")
	shortCode := shortURL[strings.LastIndex(shortURL, "/")+1:]

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
		req := httptest.NewRequest(method, "/"+shortCode, nil)
		rr := httptest.NewRecorder()
		GetLink(rr, req, shortener)
		if rr.Code != http.StatusMethodNotAllowed {
			t.Errorf("GetLink %s: got status %d, want %d", method, rr.Code, http.StatusMethodNotAllowed)
		}
	}
}
