package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zhebrikov/shortener/internal/service"
)

func TestShortenerHandler_CreateLink_POST_Success(t *testing.T) {
	shortener := service.NewShortener("localhost:8080")
	store := mustTempStorage(t, "[]")
	h := NewShortenerHandler(shortener, store)

	body := []byte("https://example.com/page")
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "text/plain")
	rr := httptest.NewRecorder()

	h.CreateLink(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("ShortenerHandler.CreateLink POST: got status %d, want %d", rr.Code, http.StatusCreated)
	}
	respBody := strings.TrimSpace(rr.Body.String())
	if !strings.HasPrefix(respBody, "http://localhost:8080/") {
		t.Errorf("ShortenerHandler.CreateLink POST: response %q does not start with base URL", respBody)
	}
}

func TestShortenerHandler_CreateLink_GET_Redirect(t *testing.T) {
	shortener := service.NewShortener("localhost:8080")
	originalURL := "https://example.com/target"
	shortURL := shortener.CreateLink(originalURL)
	shortCode := shortURL[strings.LastIndex(shortURL, "/")+1:]
	store := mustTempStorage(t, `[{"uuid":1,"short_url":"`+shortCode+`","original_url":"`+originalURL+`"}]`)
	h := NewShortenerHandler(shortener, store)

	req := httptest.NewRequest(http.MethodGet, "/"+shortCode, nil)
	rr := httptest.NewRecorder()
	h.CreateLink(rr, req)

	if rr.Code != http.StatusTemporaryRedirect {
		t.Errorf("ShortenerHandler.CreateLink GET: got status %d, want %d", rr.Code, http.StatusTemporaryRedirect)
	}
	if loc := rr.Header().Get("Location"); loc != originalURL {
		t.Errorf("ShortenerHandler.CreateLink GET: Location = %q, want %q", loc, originalURL)
	}
}

func TestShortenerHandler_CreateLink_GET_NotFound(t *testing.T) {
	shortener := service.NewShortener("localhost:8080")
	store := mustTempStorage(t, "[]")
	h := NewShortenerHandler(shortener, store)

	req := httptest.NewRequest(http.MethodGet, "/nonexistent", nil)
	rr := httptest.NewRecorder()
	h.CreateLink(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("ShortenerHandler.CreateLink GET: got status %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestShortenerHandler_CreateLink_MethodNotAllowed(t *testing.T) {
	shortener := service.NewShortener("localhost:8080")
	store := mustTempStorage(t, "[]")
	h := NewShortenerHandler(shortener, store)

	req := httptest.NewRequest(http.MethodPut, "/", nil)
	rr := httptest.NewRecorder()
	h.CreateLink(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("ShortenerHandler.CreateLink PUT: got status %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}

func TestShortenerHandler_GetLink_Redirect(t *testing.T) {
	shortener := service.NewShortener("localhost:8080")
	originalURL := "https://example.com/redirect"
	shortURL := shortener.CreateLink(originalURL)
	shortCode := shortURL[strings.LastIndex(shortURL, "/")+1:]
	store := mustTempStorage(t, `[{"uuid":1,"short_url":"`+shortCode+`","original_url":"`+originalURL+`"}]`)
	h := NewShortenerHandler(shortener, store)

	req := httptest.NewRequest(http.MethodGet, "/"+shortCode, nil)
	rr := httptest.NewRecorder()
	h.GetLink(rr, req)

	if rr.Code != http.StatusTemporaryRedirect {
		t.Errorf("ShortenerHandler.GetLink: got status %d, want %d", rr.Code, http.StatusTemporaryRedirect)
	}
	if loc := rr.Header().Get("Location"); loc != originalURL {
		t.Errorf("ShortenerHandler.GetLink: Location = %q, want %q", loc, originalURL)
	}
}

func TestShortenerHandler_GetLink_NotFound(t *testing.T) {
	shortener := service.NewShortener("localhost:8080")
	store := mustTempStorage(t, "[]")
	h := NewShortenerHandler(shortener, store)

	req := httptest.NewRequest(http.MethodGet, "/unknown", nil)
	rr := httptest.NewRecorder()
	h.GetLink(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("ShortenerHandler.GetLink: got status %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestNewShortenerHandler(t *testing.T) {
	shortener := service.NewShortener("test:9090")
	store := mustTempStorage(t, "[]")
	h := NewShortenerHandler(shortener, store)
	if h == nil {
		t.Fatal("NewShortenerHandler returned nil")
	}
	if h.shortener != shortener {
		t.Error("NewShortenerHandler: shortener not set correctly")
	}
	if h.storage != store {
		t.Error("NewShortenerHandler: storage not set correctly")
	}
}
