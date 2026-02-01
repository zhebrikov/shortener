package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zhebrikov/shortener/internal/handler"
	"github.com/zhebrikov/shortener/internal/service"
)

// handlerFromMain возвращает тот же обработчик, что регистрируется в main()
func handlerFromMain() (h *handler.ShortenerHandler) {
	shortener := service.NewShortener()
	return handler.NewShortenerHandler(shortener)
}

func TestMain_POST_CreateLink_Success(t *testing.T) {
	h := handlerFromMain()

	body := []byte("https://example.com/page")
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "text/plain")
	rr := httptest.NewRecorder()

	h.CreateLink(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("POST /: got status %d, want %d", rr.Code, http.StatusCreated)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "text/plain" {
		t.Errorf("POST /: Content-Type = %q, want text/plain", ct)
	}
	respBody := strings.TrimSpace(rr.Body.String())
	if !strings.HasPrefix(respBody, "http://localhost:8080/") {
		t.Errorf("POST /: response %q does not start with base URL", respBody)
	}
}

func TestMain_GET_RedirectWhenFound(t *testing.T) {
	h := handlerFromMain()

	// Создаём ссылку через POST
	body := []byte("https://example.com/redirect-target")
	postReq := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	postReq.Header.Set("Content-Type", "text/plain")
	postRR := httptest.NewRecorder()
	h.CreateLink(postRR, postReq)
	if postRR.Code != http.StatusCreated {
		t.Fatalf("POST: got status %d", postRR.Code)
	}
	shortURL := strings.TrimSpace(postRR.Body.String())
	shortCode := shortURL[strings.LastIndex(shortURL, "/")+1:]

	// GET по короткому коду — редирект
	getReq := httptest.NewRequest(http.MethodGet, "/"+shortCode, nil)
	getRR := httptest.NewRecorder()
	h.CreateLink(getRR, getReq)

	if getRR.Code != http.StatusTemporaryRedirect {
		t.Errorf("GET /:id: got status %d, want %d", getRR.Code, http.StatusTemporaryRedirect)
	}
	if loc := getRR.Header().Get("Location"); loc != "https://example.com/redirect-target" {
		t.Errorf("GET /:id: Location = %q, want https://example.com/redirect-target", loc)
	}
}

func TestMain_GET_NotFound(t *testing.T) {
	h := handlerFromMain()

	req := httptest.NewRequest(http.MethodGet, "/nonexistent123", nil)
	rr := httptest.NewRecorder()
	h.CreateLink(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("GET /nonexistent: got status %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestMain_MethodNotAllowed(t *testing.T) {
	h := handlerFromMain()

	methods := []string{http.MethodPut, http.MethodDelete, http.MethodPatch}
	for _, method := range methods {
		req := httptest.NewRequest(method, "/", nil)
		rr := httptest.NewRecorder()
		h.CreateLink(rr, req)
		if rr.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s /: got status %d, want %d", method, rr.Code, http.StatusMethodNotAllowed)
		}
	}
}

func TestMain_POST_NoContentType_BadRequest(t *testing.T) {
	h := handlerFromMain()

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte("https://example.com")))
	rr := httptest.NewRecorder()
	h.CreateLink(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("POST without Content-Type: got status %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestMain_POST_EmptyBody_BadRequest(t *testing.T) {
	h := handlerFromMain()

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(nil))
	req.Header.Set("Content-Type", "text/plain")
	rr := httptest.NewRecorder()
	h.CreateLink(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("POST empty body: got status %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestMain_GET_RootPath_NotFound(t *testing.T) {
	h := handlerFromMain()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	h.CreateLink(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("GET /: got status %d, want %d", rr.Code, http.StatusNotFound)
	}
}
