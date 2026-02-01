package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zhebrikov/shortener/internal/service"
)

func TestGetLink_Success(t *testing.T) {
	shortener := service.NewShortener()
	targetURL := "https://example.com/target"
	shortCode := shortener.CreateLink(targetURL)

	req := httptest.NewRequest(http.MethodGet, "/"+shortCode, nil)
	rr := httptest.NewRecorder()

	GetLink(rr, req, shortener)

	if rr.Code != http.StatusTemporaryRedirect {
		t.Errorf("GetLink: got status %d, want %d", rr.Code, http.StatusTemporaryRedirect)
	}
	loc := rr.Header().Get("Location")
	if loc != targetURL {
		t.Errorf("GetLink: Location = %q, want %q", loc, targetURL)
	}
}

func TestGetLink_NotFound(t *testing.T) {
	shortener := service.NewShortener()
	req := httptest.NewRequest(http.MethodGet, "/nonexistent123", nil)
	rr := httptest.NewRecorder()

	GetLink(rr, req, shortener)

	if rr.Code != http.StatusNotFound {
		t.Errorf("GetLink: got status %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestGetLink_EmptyShortCode(t *testing.T) {
	shortener := service.NewShortener()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	GetLink(rr, req, shortener)

	if rr.Code != http.StatusNotFound {
		t.Errorf("GetLink (empty path): got status %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestGetLink_MethodNotAllowed(t *testing.T) {
	shortener := service.NewShortener()
	shortCode := shortener.CreateLink("https://example.com")
	methods := []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch, http.MethodOptions}

	for _, method := range methods {
		req := httptest.NewRequest(method, "/"+shortCode, nil)
		rr := httptest.NewRecorder()
		GetLink(rr, req, shortener)
		if rr.Code != http.StatusMethodNotAllowed {
			t.Errorf("GetLink(%s): got status %d, want %d", method, rr.Code, http.StatusMethodNotAllowed)
		}
	}
}

func TestGetLink_InvalidShortCodeFormat(t *testing.T) {
	shortener := service.NewShortener()
	req := httptest.NewRequest(http.MethodGet, "/not-a-valid-hex-code!!!", nil)
	rr := httptest.NewRecorder()

	GetLink(rr, req, shortener)

	if rr.Code != http.StatusNotFound {
		t.Errorf("GetLink: got status %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestGetLink_PathWithTrailingSlash(t *testing.T) {
	shortener := service.NewShortener()
	shortCode := shortener.CreateLink("https://example.com")
	req := httptest.NewRequest(http.MethodGet, "/"+shortCode+"/", nil)
	rr := httptest.NewRecorder()

	GetLink(rr, req, shortener)

	// r.URL.Path = "/abc12345/" -> Path[1:] = "abc12345/" — service won't find "abc12345/"
	if rr.Code != http.StatusNotFound {
		t.Errorf("GetLink (trailing slash): got status %d, want %d", rr.Code, http.StatusNotFound)
	}
}
