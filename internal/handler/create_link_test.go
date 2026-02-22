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
	shortener := service.NewShortener("localhost:8080")
	store := mustTempStorage(t, "[]")

	body := []byte("https://example.com/page")
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "text/plain")
	rr := httptest.NewRecorder()

	CreateLink(rr, req, shortener, store)

	if rr.Code != http.StatusCreated {
		t.Errorf("CreateLink: got status %d, want %d", rr.Code, http.StatusCreated)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "text/plain" {
		t.Errorf("CreateLink: Content-Type = %q, want text/plain", ct)
	}
	respBody := strings.TrimSpace(rr.Body.String())
	if !strings.HasPrefix(respBody, "http://localhost:8080/") {
		t.Errorf("CreateLink: response %q does not start with base URL", respBody)
	}
	if len(respBody) < len("http://localhost:8080/")+8 {
		t.Errorf("CreateLink: short code too short")
	}
}

func TestCreateLink_MethodNotAllowed(t *testing.T) {
	shortener := service.NewShortener("localhost:8080")
	store := mustTempStorage(t, "[]")

	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
		req := httptest.NewRequest(method, "/", bytes.NewReader([]byte("https://example.com")))
		req.Header.Set("Content-Type", "text/plain")
		rr := httptest.NewRecorder()
		CreateLink(rr, req, shortener, store)
		if rr.Code != http.StatusMethodNotAllowed {
			t.Errorf("CreateLink %s: got status %d, want %d", method, rr.Code, http.StatusMethodNotAllowed)
		}
		if body := rr.Body.String(); !strings.Contains(body, "Method not allowed") {
			t.Errorf("CreateLink %s: body should mention method not allowed", method)
		}
	}
}

func TestCreateLink_ContentTypeRequired(t *testing.T) {
	shortener := service.NewShortener("localhost:8080")
	store := mustTempStorage(t, "[]")

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte("https://example.com")))
	rr := httptest.NewRecorder()
	CreateLink(rr, req, shortener, store)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("CreateLink without Content-Type: got status %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestCreateLink_EmptyBody_BadRequest(t *testing.T) {
	shortener := service.NewShortener("localhost:8080")
	store := mustTempStorage(t, "[]")

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(nil))
	req.Header.Set("Content-Type", "text/plain")
	rr := httptest.NewRecorder()
	CreateLink(rr, req, shortener, store)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("CreateLink empty body: got status %d, want %d", rr.Code, http.StatusBadRequest)
	}
	if body := rr.Body.String(); !strings.Contains(body, "Invalid request body") {
		t.Errorf("CreateLink empty body: body should mention invalid request body")
	}
}
