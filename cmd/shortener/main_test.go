package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/zhebrikov/shortener/internal/handler"
	"github.com/zhebrikov/shortener/internal/service"
)

func handlerFromMain() (h *handler.ShortenerHandler) {
	shortener := service.NewShortener("localhost:8080")
	return handler.NewShortenerHandler(shortener)
}

// routerFromMain возвращает роутер в том же виде, что и в main (для интеграционных тестов).
func routerFromMain(baseURL string) http.Handler {
	shortener := service.NewShortener(baseURL)
	h := handler.NewShortenerHandler(shortener)
	r := chi.NewRouter()
	r.Post("/", h.CreateLink)
	r.Get("/{shortCode}", h.GetLink)
	return r
}

func TestGetConfig(t *testing.T) {
	const defaultAddr = "localhost:8080"
	const defaultBase = "http://example.com"

	saveEnv := func(key string) (string, bool) {
		v, ok := os.LookupEnv(key)
		return v, ok
	}
	restoreEnv := func(key, value string, had bool) {
		if had {
			os.Setenv(key, value)
		} else {
			os.Unsetenv(key)
		}
	}

	t.Run("no env uses flags", func(t *testing.T) {
		oldAddr, addrOk := saveEnv("SERVER_ADDRESS")
		oldBase, baseOk := saveEnv("BASE_URL")
		os.Unsetenv("SERVER_ADDRESS")
		os.Unsetenv("BASE_URL")
		defer restoreEnv("SERVER_ADDRESS", oldAddr, addrOk)
		defer restoreEnv("BASE_URL", oldBase, baseOk)

		gotAddr, gotBase := getConfig(defaultAddr, defaultBase)
		if gotAddr != defaultAddr || gotBase != defaultBase {
			t.Errorf("getConfig() = %q, %q; want %q, %q", gotAddr, gotBase, defaultAddr, defaultBase)
		}
	})

	t.Run("SERVER_ADDRESS overrides flag", func(t *testing.T) {
		oldAddr, addrOk := saveEnv("SERVER_ADDRESS")
		oldBase, baseOk := saveEnv("BASE_URL")
		os.Setenv("SERVER_ADDRESS", "0.0.0.0:9090")
		os.Unsetenv("BASE_URL")
		defer restoreEnv("SERVER_ADDRESS", oldAddr, addrOk)
		defer restoreEnv("BASE_URL", oldBase, baseOk)

		gotAddr, gotBase := getConfig(defaultAddr, defaultBase)
		if gotAddr != "0.0.0.0:9090" {
			t.Errorf("getConfig() serverAddress = %q; want 0.0.0.0:9090", gotAddr)
		}
		if gotBase != defaultBase {
			t.Errorf("getConfig() baseURL = %q; want %q", gotBase, defaultBase)
		}
	})

	t.Run("BASE_URL overrides flag", func(t *testing.T) {
		oldAddr, addrOk := saveEnv("SERVER_ADDRESS")
		oldBase, baseOk := saveEnv("BASE_URL")
		os.Unsetenv("SERVER_ADDRESS")
		os.Setenv("BASE_URL", "https://short.example.com")
		defer restoreEnv("SERVER_ADDRESS", oldAddr, addrOk)
		defer restoreEnv("BASE_URL", oldBase, baseOk)

		gotAddr, gotBase := getConfig(defaultAddr, defaultBase)
		if gotAddr != defaultAddr {
			t.Errorf("getConfig() serverAddress = %q; want %q", gotAddr, defaultAddr)
		}
		if gotBase != "https://short.example.com" {
			t.Errorf("getConfig() baseURL = %q; want https://short.example.com", gotBase)
		}
	})

	t.Run("both env override flags", func(t *testing.T) {
		oldAddr, addrOk := saveEnv("SERVER_ADDRESS")
		oldBase, baseOk := saveEnv("BASE_URL")
		os.Setenv("SERVER_ADDRESS", ":3000")
		os.Setenv("BASE_URL", "https://s.example.com")
		defer restoreEnv("SERVER_ADDRESS", oldAddr, addrOk)
		defer restoreEnv("BASE_URL", oldBase, baseOk)

		gotAddr, gotBase := getConfig(defaultAddr, defaultBase)
		if gotAddr != ":3000" || gotBase != "https://s.example.com" {
			t.Errorf("getConfig() = %q, %q; want :3000, https://s.example.com", gotAddr, gotBase)
		}
	})
}

func TestPortFromServerAddress(t *testing.T) {
	tests := []struct {
		name    string
		addr    string
		want    string
		wantErr bool
	}{
		{"host and port", "localhost:8080", ":8080", false},
		{"only port", ":9090", ":9090", false},
		{"no port", "localhost", "", true},
		{"empty", "", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := portFromServerAddress(tt.addr)
			if (err != nil) != tt.wantErr {
				t.Errorf("portFromServerAddress() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("portFromServerAddress() = %q, want %q", got, tt.want)
			}
		})
	}
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
	h.GetLink(getRR, getReq)

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
	h.GetLink(rr, req)

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
	h.GetLink(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("GET /: got status %d, want %d", rr.Code, http.StatusNotFound)
	}
}

// Тесты через роутер chi как в main — проверяют фактическую маршрутизацию приложения.

func TestRouter_POST_CreateLink_Success(t *testing.T) {
	baseURL := "localhost:8080"
	r := routerFromMain(baseURL)

	body := []byte("https://example.com/page")
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "text/plain")
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("POST /: got status %d, want %d", rr.Code, http.StatusCreated)
	}
	respBody := strings.TrimSpace(rr.Body.String())
	if !strings.HasPrefix(respBody, "http://"+baseURL+"/") {
		t.Errorf("POST /: response %q does not start with base URL", respBody)
	}
}

func TestRouter_GET_RedirectWhenFound(t *testing.T) {
	r := routerFromMain("localhost:8080")

	body := []byte("https://example.com/redirect-target")
	postReq := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	postReq.Header.Set("Content-Type", "text/plain")
	postRR := httptest.NewRecorder()
	r.ServeHTTP(postRR, postReq)
	if postRR.Code != http.StatusCreated {
		t.Fatalf("POST: got status %d", postRR.Code)
	}
	shortURL := strings.TrimSpace(postRR.Body.String())
	shortCode := shortURL[strings.LastIndex(shortURL, "/")+1:]

	getReq := httptest.NewRequest(http.MethodGet, "/"+shortCode, nil)
	getRR := httptest.NewRecorder()
	r.ServeHTTP(getRR, getReq)

	if getRR.Code != http.StatusTemporaryRedirect {
		t.Errorf("GET /%s: got status %d, want %d", shortCode, getRR.Code, http.StatusTemporaryRedirect)
	}
	if loc := getRR.Header().Get("Location"); loc != "https://example.com/redirect-target" {
		t.Errorf("GET /%s: Location = %q, want https://example.com/redirect-target", shortCode, loc)
	}
}

func TestRouter_GET_NotFound(t *testing.T) {
	r := routerFromMain("localhost:8080")

	req := httptest.NewRequest(http.MethodGet, "/nonexistent123", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("GET /nonexistent123: got status %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestRouter_GET_RootPath_NotFound(t *testing.T) {
	r := routerFromMain("localhost:8080")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	// chi не матчит GET "/" с маршрутом "/{shortCode}", возвращает 405
	if rr.Code != http.StatusMethodNotAllowed && rr.Code != http.StatusNotFound {
		t.Errorf("GET /: got status %d, want 404 or 405", rr.Code)
	}
}

func TestRouter_POST_EmptyBody_BadRequest(t *testing.T) {
	r := routerFromMain("localhost:8080")

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(nil))
	req.Header.Set("Content-Type", "text/plain")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("POST empty body: got status %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestRouter_POST_NoContentType_BadRequest(t *testing.T) {
	r := routerFromMain("localhost:8080")

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte("https://example.com")))
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("POST without Content-Type: got status %d, want %d", rr.Code, http.StatusBadRequest)
	}
}
