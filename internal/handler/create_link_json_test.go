package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zhebrikov/shortener/internal/service"
)

func TestCreateLinkJson_Success(t *testing.T) {
	shortener := service.NewShortener("localhost:8080")

	body, _ := json.Marshal(Input{URL: "https://example.com/page"})
	req := httptest.NewRequest(http.MethodPost, "/api/shorten", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	CreateLinkJson(rr, req, shortener)

	if rr.Code != http.StatusCreated {
		t.Errorf("CreateLinkJson: got status %d, want %d", rr.Code, http.StatusCreated)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("CreateLinkJson: Content-Type = %q, want application/json", ct)
	}
	var out Output
	if err := json.NewDecoder(rr.Body).Decode(&out); err != nil {
		t.Fatalf("CreateLinkJson: invalid JSON response: %v", err)
	}
	if !strings.HasPrefix(out.Result, "http://localhost:8080/") {
		t.Errorf("CreateLinkJson: response result %q does not start with base URL", out.Result)
	}
	if len(out.Result) < len("http://localhost:8080/")+8 {
		t.Errorf("CreateLinkJson: short code too short")
	}
}

func TestCreateLinkJson_MethodNotAllowed(t *testing.T) {
	shortener := service.NewShortener("localhost:8080")

	body, _ := json.Marshal(Input{URL: "https://example.com"})
	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
		req := httptest.NewRequest(method, "/api/shorten", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		CreateLinkJson(rr, req, shortener)
		if rr.Code != http.StatusMethodNotAllowed {
			t.Errorf("CreateLinkJson %s: got status %d, want %d", method, rr.Code, http.StatusMethodNotAllowed)
		}
		if body := rr.Body.String(); !strings.Contains(body, "Method not allowed") {
			t.Errorf("CreateLinkJson %s: body should mention method not allowed", method)
		}
	}
}

func TestCreateLinkJson_InvalidJSON_BadRequest(t *testing.T) {
	shortener := service.NewShortener("localhost:8080")

	req := httptest.NewRequest(http.MethodPost, "/api/shorten", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	CreateLinkJson(rr, req, shortener)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("CreateLinkJson invalid JSON: got status %d, want %d", rr.Code, http.StatusBadRequest)
	}
	if body := rr.Body.String(); !strings.Contains(body, "Invalid request body") {
		t.Errorf("CreateLinkJson invalid JSON: body should mention invalid request body")
	}
}

func TestCreateLinkJson_MalformedJSON_BadRequest(t *testing.T) {
	shortener := service.NewShortener("localhost:8080")

	req := httptest.NewRequest(http.MethodPost, "/api/shorten", bytes.NewReader([]byte(`{"url": }`)))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	CreateLinkJson(rr, req, shortener)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("CreateLinkJson malformed JSON: got status %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestCreateLinkJson_EmptyURL_ReturnsShortURL(t *testing.T) {
	shortener := service.NewShortener("localhost:8080")

	body, _ := json.Marshal(Input{URL: ""})
	req := httptest.NewRequest(http.MethodPost, "/api/shorten", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	CreateLinkJson(rr, req, shortener)

	if rr.Code != http.StatusCreated {
		t.Errorf("CreateLinkJson empty URL: got status %d, want %d", rr.Code, http.StatusCreated)
	}
	var out Output
	if err := json.NewDecoder(rr.Body).Decode(&out); err != nil {
		t.Fatalf("CreateLinkJson empty URL: invalid JSON response: %v", err)
	}
	if !strings.HasPrefix(out.Result, "http://localhost:8080/") {
		t.Errorf("CreateLinkJson empty URL: response result %q does not start with base URL", out.Result)
	}
}
