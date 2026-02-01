package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zhebrikov/shortener/internal/service"
)

func TestNewShortenerHandler(t *testing.T) {
	s := service.NewShortener()
	h := NewShortenerHandler(s)
	if h == nil {
		t.Fatal("NewShortenerHandler returned nil")
	}
	if h.shortener != s {
		t.Error("NewShortenerHandler: shortener not set correctly")
	}
}

func TestShortenerHandler_CreateLink_POST(t *testing.T) {
	s := service.NewShortener()
	h := NewShortenerHandler(s)

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte("https://example.com")))
	req.Header.Set("Content-Type", "text/plain")
	rr := httptest.NewRecorder()

	h.CreateLink(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("CreateLink POST: got status %d, want %d", rr.Code, http.StatusCreated)
	}
}

func TestShortenerHandler_CreateLink_GET_RedirectsWhenFound(t *testing.T) {
	s := service.NewShortener()
	targetURL := "https://example.com/foo"
	shortCode := s.CreateLink(targetURL)
	h := NewShortenerHandler(s)

	req := httptest.NewRequest(http.MethodGet, "/"+shortCode, nil)
	rr := httptest.NewRecorder()

	h.CreateLink(rr, req)

	if rr.Code != http.StatusTemporaryRedirect {
		t.Errorf("CreateLink GET (found): got status %d, want %d", rr.Code, http.StatusTemporaryRedirect)
	}
	if loc := rr.Header().Get("Location"); loc != targetURL {
		t.Errorf("CreateLink GET: Location = %q, want %q", loc, targetURL)
	}
}

func TestShortenerHandler_CreateLink_GET_NotFound(t *testing.T) {
	s := service.NewShortener()
	h := NewShortenerHandler(s)

	req := httptest.NewRequest(http.MethodGet, "/nonexistent", nil)
	rr := httptest.NewRecorder()

	h.CreateLink(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("CreateLink GET (not found): got status %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestShortenerHandler_CreateLink_MethodNotAllowed(t *testing.T) {
	s := service.NewShortener()
	h := NewShortenerHandler(s)

	methods := []string{http.MethodPut, http.MethodDelete, http.MethodPatch, http.MethodOptions}

	for _, method := range methods {
		req := httptest.NewRequest(method, "/", nil)
		rr := httptest.NewRecorder()
		h.CreateLink(rr, req)
		if rr.Code != http.StatusMethodNotAllowed {
			t.Errorf("CreateLink(%s): got status %d, want %d", method, rr.Code, http.StatusMethodNotAllowed)
		}
	}
}

func TestShortenerHandler_GetLink_Success(t *testing.T) {
	s := service.NewShortener()
	targetURL := "https://example.com/bar"
	shortCode := s.CreateLink(targetURL)
	h := NewShortenerHandler(s)

	req := httptest.NewRequest(http.MethodGet, "/"+shortCode, nil)
	rr := httptest.NewRecorder()

	h.GetLink(rr, req)

	if rr.Code != http.StatusTemporaryRedirect {
		t.Errorf("GetLink: got status %d, want %d", rr.Code, http.StatusTemporaryRedirect)
	}
	if loc := rr.Header().Get("Location"); loc != targetURL {
		t.Errorf("GetLink: Location = %q, want %q", loc, targetURL)
	}
}

func TestShortenerHandler_GetLink_NotFound(t *testing.T) {
	s := service.NewShortener()
	h := NewShortenerHandler(s)

	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	rr := httptest.NewRecorder()

	h.GetLink(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("GetLink: got status %d, want %d", rr.Code, http.StatusNotFound)
	}
}
