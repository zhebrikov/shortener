package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zhebrikov/shortener/internal/service"
)

func TestCreateLink_Success(t *testing.T) {
	shortener := service.NewShortener()
	body := []byte("https://example.com/page")
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "text/plain")
	rr := httptest.NewRecorder()

	CreateLink(rr, req, shortener)

	if rr.Code != http.StatusCreated {
		t.Errorf("CreateLink: got status %d, want %d", rr.Code, http.StatusCreated)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "text/plain" {
		t.Errorf("CreateLink: Content-Type = %q, want text/plain", ct)
	}
	respBody := strings.TrimSpace(rr.Body.String())
	if respBody == "" {
		t.Error("CreateLink: response body is empty")
	}
	if !strings.HasPrefix(respBody, "http://localhost:8080/") {
		t.Errorf("CreateLink: response %q does not start with http://localhost:8080/", respBody)
	}
	if len(respBody) <= len("http://localhost:8080/") {
		t.Error("CreateLink: short code is missing in response")
	}
}

func TestCreateLink_MethodNotAllowed(t *testing.T) {
	shortener := service.NewShortener()
	methods := []string{http.MethodGet, http.MethodPut, http.MethodDelete, http.MethodPatch, http.MethodOptions, "INVALID"}

	for _, method := range methods {
		req := httptest.NewRequest(method, "/", bytes.NewReader([]byte("https://example.com")))
		req.Header.Set("Content-Type", "text/plain")
		rr := httptest.NewRecorder()
		CreateLink(rr, req, shortener)
		if rr.Code != http.StatusMethodNotAllowed {
			t.Errorf("CreateLink(%s): got status %d, want %d", method, rr.Code, http.StatusMethodNotAllowed)
		}
	}
}

func TestCreateLink_WrongContentType(t *testing.T) {
	shortener := service.NewShortener()
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte("https://example.com")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	CreateLink(rr, req, shortener)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("CreateLink: got status %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestCreateLink_EmptyBody(t *testing.T) {
	shortener := service.NewShortener()
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(nil))
	req.Header.Set("Content-Type", "text/plain")
	rr := httptest.NewRecorder()

	CreateLink(rr, req, shortener)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("CreateLink: got status %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestCreateLink_NoContentType(t *testing.T) {
	shortener := service.NewShortener()
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte("https://example.com")))
	rr := httptest.NewRecorder()

	CreateLink(rr, req, shortener)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("CreateLink: got status %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestCreateLink_SpecialCharactersInURL(t *testing.T) {
	shortener := service.NewShortener()
	body := []byte("https://example.com/path?q=1&foo=bar#anchor")
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "text/plain")
	rr := httptest.NewRecorder()

	CreateLink(rr, req, shortener)

	if rr.Code != http.StatusCreated {
		t.Errorf("CreateLink: got status %d, want %d", rr.Code, http.StatusCreated)
	}
	if !strings.HasPrefix(rr.Body.String(), "http://localhost:8080/") {
		t.Errorf("CreateLink: unexpected response %q", rr.Body.String())
	}
}

func TestCreateLink_VeryLongURL(t *testing.T) {
	shortener := service.NewShortener()
	longURL := "https://example.com/" + strings.Repeat("a", 10000)
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(longURL))
	req.Header.Set("Content-Type", "text/plain")
	rr := httptest.NewRecorder()

	CreateLink(rr, req, shortener)

	if rr.Code != http.StatusCreated {
		t.Errorf("CreateLink: got status %d, want %d", rr.Code, http.StatusCreated)
	}
}

func TestCreateLink_SameURLReturnsSameShortCode(t *testing.T) {
	shortener := service.NewShortener()
	url := "https://example.com/unique"
	req1 := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte(url)))
	req1.Header.Set("Content-Type", "text/plain")
	rr1 := httptest.NewRecorder()
	CreateLink(rr1, req1, shortener)

	req2 := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte(url)))
	req2.Header.Set("Content-Type", "text/plain")
	rr2 := httptest.NewRecorder()
	CreateLink(rr2, req2, shortener)

	if rr1.Code != http.StatusCreated || rr2.Code != http.StatusCreated {
		t.Fatalf("CreateLink: status %d, %d", rr1.Code, rr2.Code)
	}
	if rr1.Body.String() != rr2.Body.String() {
		t.Errorf("CreateLink: same URL should return same short URL: %q vs %q", rr1.Body.String(), rr2.Body.String())
	}
}
