package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/zhebrikov/shortener/internal/handler"
	"github.com/zhebrikov/shortener/internal/logger"
	"github.com/zhebrikov/shortener/internal/middleware"
	"github.com/zhebrikov/shortener/internal/service"
	"github.com/zhebrikov/shortener/internal/storage"
	"go.uber.org/zap"
)

// storageFromTestFile создаёт временный JSON-файл для тестов storage (пустой массив).
func storageFromTestFile(t *testing.T) *storage.Storage {
	t.Helper()
	f, err := os.CreateTemp("", "shortener_*.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("[]"); err != nil {
		f.Close()
		os.Remove(f.Name())
		t.Fatal(err)
	}
	f.Close()
	return storage.NewStorage(f.Name())
}

func handlerFromMain(t *testing.T) *handler.ShortenerHandler {
	t.Helper()
	shortener := service.NewShortener("localhost:8080")
	store := storageFromTestFile(t)
	return handler.NewShortenerHandler(shortener, store)
}

// routerFromMain возвращает роутер в том же виде, что и в main (для интеграционных тестов).
func routerFromMain(t *testing.T, baseURL string) http.Handler {
	t.Helper()
	shortener := service.NewShortener(baseURL)
	store := storageFromTestFile(t)
	h := handler.NewShortenerHandler(shortener, store)
	r := chi.NewRouter()
	r.Use(logger.Middleware(zap.NewNop()))
	r.Use(middleware.Gzip)
	r.Post("/", h.CreateLink)
	r.Get("/{shortCode}", h.GetLink)
	r.Post("/api/shorten", h.CreateLinkJSON)
	r.Post("/api/shorten/batch", h.CreateLinkBatch)
	return r
}

func TestGetConfig(t *testing.T) {
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

	defaultCfg := Config{
		ServerAddress: "localhost:8080",
		BaseURL:       "http://example.com",
		FileStorage:   "",
		DatabaseDsn:   "",
	}

	t.Run("missing SERVER_ADDRESS uses default", func(t *testing.T) {
		oldAddr, addrOk := saveEnv("SERVER_ADDRESS")
		oldBase, baseOk := saveEnv("BASE_URL")
		oldFile, fileOk := saveEnv("FILE_STORAGE_PATH")
		os.Unsetenv("SERVER_ADDRESS")
		os.Setenv("BASE_URL", "http://example.com")
		os.Setenv("FILE_STORAGE_PATH", "file.json")
		defer restoreEnv("SERVER_ADDRESS", oldAddr, addrOk)
		defer restoreEnv("BASE_URL", oldBase, baseOk)
		defer restoreEnv("FILE_STORAGE_PATH", oldFile, fileOk)

		got, err := getConfig(defaultCfg)
		if err != nil {
			t.Fatalf("getConfig() unexpected error: %v", err)
		}
		if got.ServerAddress != defaultCfg.ServerAddress {
			t.Errorf("getConfig() ServerAddress = %q; want %q (default)", got.ServerAddress, defaultCfg.ServerAddress)
		}
	})

	t.Run("missing BASE_URL uses default", func(t *testing.T) {
		oldAddr, addrOk := saveEnv("SERVER_ADDRESS")
		oldBase, baseOk := saveEnv("BASE_URL")
		oldFile, fileOk := saveEnv("FILE_STORAGE_PATH")
		os.Setenv("SERVER_ADDRESS", "localhost:8080")
		os.Unsetenv("BASE_URL")
		os.Setenv("FILE_STORAGE_PATH", "file.json")
		defer restoreEnv("SERVER_ADDRESS", oldAddr, addrOk)
		defer restoreEnv("BASE_URL", oldBase, baseOk)
		defer restoreEnv("FILE_STORAGE_PATH", oldFile, fileOk)

		got, err := getConfig(defaultCfg)
		if err != nil {
			t.Fatalf("getConfig() unexpected error: %v", err)
		}
		if got.BaseURL != defaultCfg.BaseURL {
			t.Errorf("getConfig() BaseURL = %q; want %q (default)", got.BaseURL, defaultCfg.BaseURL)
		}
	})

	t.Run("missing FILE_STORAGE_PATH uses default (empty)", func(t *testing.T) {
		oldAddr, addrOk := saveEnv("SERVER_ADDRESS")
		oldBase, baseOk := saveEnv("BASE_URL")
		oldFile, fileOk := saveEnv("FILE_STORAGE_PATH")
		os.Setenv("SERVER_ADDRESS", "localhost:8080")
		os.Setenv("BASE_URL", "http://example.com")
		os.Unsetenv("FILE_STORAGE_PATH")
		defer restoreEnv("SERVER_ADDRESS", oldAddr, addrOk)
		defer restoreEnv("BASE_URL", oldBase, baseOk)
		defer restoreEnv("FILE_STORAGE_PATH", oldFile, fileOk)

		got, err := getConfig(defaultCfg)
		if err != nil {
			t.Fatalf("getConfig() unexpected error: %v", err)
		}
		if got.FileStorage != "" {
			t.Errorf("getConfig() FileStorage = %q; want %q (default)", got.FileStorage, "")
		}
	})

	t.Run("all env set returns config from env", func(t *testing.T) {
		oldAddr, addrOk := saveEnv("SERVER_ADDRESS")
		oldBase, baseOk := saveEnv("BASE_URL")
		oldFile, fileOk := saveEnv("FILE_STORAGE_PATH")
		oldDsn, dsnOk := saveEnv("DATABASE_DSN")
		os.Setenv("SERVER_ADDRESS", "0.0.0.0:9090")
		os.Setenv("BASE_URL", "https://short.example.com")
		os.Setenv("FILE_STORAGE_PATH", "/tmp/links.json")
		os.Unsetenv("DATABASE_DSN")
		defer restoreEnv("SERVER_ADDRESS", oldAddr, addrOk)
		defer restoreEnv("BASE_URL", oldBase, baseOk)
		defer restoreEnv("FILE_STORAGE_PATH", oldFile, fileOk)
		defer restoreEnv("DATABASE_DSN", oldDsn, dsnOk)

		got, err := getConfig(defaultCfg)
		if err != nil {
			t.Fatalf("getConfig() unexpected error: %v", err)
		}
		if got.ServerAddress != "0.0.0.0:9090" {
			t.Errorf("getConfig() ServerAddress = %q; want 0.0.0.0:9090", got.ServerAddress)
		}
		if got.BaseURL != "https://short.example.com" {
			t.Errorf("getConfig() BaseURL = %q; want https://short.example.com", got.BaseURL)
		}
		if got.FileStorage != "/tmp/links.json" {
			t.Errorf("getConfig() FileStorage = %q; want /tmp/links.json", got.FileStorage)
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
	h := handlerFromMain(t)

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
	h := handlerFromMain(t)

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
	h := handlerFromMain(t)

	req := httptest.NewRequest(http.MethodGet, "/nonexistent123", nil)
	rr := httptest.NewRecorder()
	h.GetLink(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("GET /nonexistent: got status %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestMain_MethodNotAllowed(t *testing.T) {
	r := routerFromMain(t, "localhost:8080")

	methods := []string{http.MethodPut, http.MethodDelete, http.MethodPatch}
	for _, method := range methods {
		req := httptest.NewRequest(method, "/", nil)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		if rr.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s /: got status %d, want %d", method, rr.Code, http.StatusMethodNotAllowed)
		}
	}
}

func TestMain_POST_NoContentType_BadRequest(t *testing.T) {
	h := handlerFromMain(t)

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte("https://example.com")))
	rr := httptest.NewRecorder()
	h.CreateLink(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("POST without Content-Type: got status %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestMain_POST_EmptyBody_BadRequest(t *testing.T) {
	h := handlerFromMain(t)

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(nil))
	req.Header.Set("Content-Type", "text/plain")
	rr := httptest.NewRecorder()
	h.CreateLink(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("POST empty body: got status %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestMain_GET_RootPath_NotFound(t *testing.T) {
	h := handlerFromMain(t)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	h.GetLink(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("GET /: got status %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestMain_POST_ApiShorten_Success(t *testing.T) {
	h := handlerFromMain(t)

	body, _ := json.Marshal(map[string]string{"url": "https://example.com/page"})
	req := httptest.NewRequest(http.MethodPost, "/api/shorten", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.CreateLinkJSON(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("POST /api/shorten: got status %d, want %d", rr.Code, http.StatusCreated)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("POST /api/shorten: Content-Type = %q, want application/json", ct)
	}
	var out struct {
		Result string `json:"result"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&out); err != nil {
		t.Fatalf("POST /api/shorten: invalid JSON response: %v", err)
	}
	if !strings.HasPrefix(out.Result, "http://localhost:8080/") {
		t.Errorf("POST /api/shorten: response url %q does not start with base URL", out.Result)
	}
}

func TestMain_POST_ApiShorten_InvalidJSON_BadRequest(t *testing.T) {
	h := handlerFromMain(t)

	req := httptest.NewRequest(http.MethodPost, "/api/shorten", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.CreateLinkJSON(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("POST /api/shorten invalid JSON: got status %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestMain_POST_ApiShorten_MethodNotAllowed(t *testing.T) {
	r := routerFromMain(t, "localhost:8080")

	body, _ := json.Marshal(map[string]string{"url": "https://example.com"})
	req := httptest.NewRequest(http.MethodGet, "/api/shorten", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET /api/shorten: got status %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}

// Тесты через роутер chi как в main — проверяют фактическую маршрутизацию приложения.

func TestRouter_POST_CreateLink_Success(t *testing.T) {
	baseURL := "localhost:8080"
	r := routerFromMain(t, baseURL)

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
	r := routerFromMain(t, "localhost:8080")

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
	r := routerFromMain(t, "localhost:8080")

	req := httptest.NewRequest(http.MethodGet, "/nonexistent123", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("GET /nonexistent123: got status %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestRouter_GET_RootPath_NotFound(t *testing.T) {
	r := routerFromMain(t, "localhost:8080")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	// chi не матчит GET "/" с маршрутом "/{shortCode}", возвращает 405
	if rr.Code != http.StatusMethodNotAllowed && rr.Code != http.StatusNotFound {
		t.Errorf("GET /: got status %d, want 404 or 405", rr.Code)
	}
}

func TestRouter_POST_EmptyBody_BadRequest(t *testing.T) {
	r := routerFromMain(t, "localhost:8080")

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(nil))
	req.Header.Set("Content-Type", "text/plain")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("POST empty body: got status %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestRouter_POST_NoContentType_BadRequest(t *testing.T) {
	r := routerFromMain(t, "localhost:8080")

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte("https://example.com")))
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("POST without Content-Type: got status %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestRouter_POST_ApiShorten_Success(t *testing.T) {
	baseURL := "localhost:8080"
	r := routerFromMain(t, baseURL)

	body, _ := json.Marshal(map[string]string{"url": "https://example.com/page"})
	req := httptest.NewRequest(http.MethodPost, "/api/shorten", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("POST /api/shorten: got status %d, want %d", rr.Code, http.StatusCreated)
	}
	var out struct {
		Result string `json:"result"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&out); err != nil {
		t.Fatalf("POST /api/shorten: invalid JSON response: %v", err)
	}
	if !strings.HasPrefix(out.Result, "http://"+baseURL+"/") {
		t.Errorf("POST /api/shorten: response url %q does not start with base URL", out.Result)
	}
}

func TestRouter_POST_ApiShorten_InvalidJSON_BadRequest(t *testing.T) {
	r := routerFromMain(t, "localhost:8080")

	req := httptest.NewRequest(http.MethodPost, "/api/shorten", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("POST /api/shorten invalid JSON: got status %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestRouter_POST_ApiShorten_ThenRedirect(t *testing.T) {
	r := routerFromMain(t, "localhost:8080")

	body, _ := json.Marshal(map[string]string{"url": "https://example.com/from-json"})
	postReq := httptest.NewRequest(http.MethodPost, "/api/shorten", bytes.NewReader(body))
	postReq.Header.Set("Content-Type", "application/json")
	postRR := httptest.NewRecorder()
	r.ServeHTTP(postRR, postReq)
	if postRR.Code != http.StatusCreated {
		t.Fatalf("POST /api/shorten: got status %d", postRR.Code)
	}
	var out struct {
		Result string `json:"result"`
	}
	if err := json.NewDecoder(postRR.Body).Decode(&out); err != nil {
		t.Fatalf("POST /api/shorten: invalid JSON: %v", err)
	}
	shortCode := out.Result[strings.LastIndex(out.Result, "/")+1:]

	getReq := httptest.NewRequest(http.MethodGet, "/"+shortCode, nil)
	getRR := httptest.NewRecorder()
	r.ServeHTTP(getRR, getReq)

	if getRR.Code != http.StatusTemporaryRedirect {
		t.Errorf("GET /%s: got status %d, want %d", shortCode, getRR.Code, http.StatusTemporaryRedirect)
	}
	if loc := getRR.Header().Get("Location"); loc != "https://example.com/from-json" {
		t.Errorf("GET /%s: Location = %q, want https://example.com/from-json", shortCode, loc)
	}
}

func TestRouter_POST_ApiShortenBatch(t *testing.T) {
	r := routerFromMain(t, "localhost:8080")

	body, _ := json.Marshal([]map[string]string{
		{"correlation_id": "id1", "original_url": "https://example.com/batch-a"},
		{"correlation_id": "id2", "original_url": "https://example.com/batch-b"},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/shorten/batch", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("POST /api/shorten/batch: got status %d, want %d", rr.Code, http.StatusCreated)
	}
	var out []struct {
		CorrelationID string `json:"correlation_id"`
		ShortURL      string `json:"short_url"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&out); err != nil {
		t.Fatalf("POST /api/shorten/batch: invalid JSON: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("POST /api/shorten/batch: len(response) = %d, want 2", len(out))
	}
	if out[0].CorrelationID != "id1" || out[1].CorrelationID != "id2" {
		t.Errorf("correlation_id: got %q, %q", out[0].CorrelationID, out[1].CorrelationID)
	}
	for i := range out {
		if !strings.HasPrefix(out[i].ShortURL, "http://localhost:8080/") {
			t.Errorf("short_url[%d] = %q, want prefix http://localhost:8080/", i, out[i].ShortURL)
		}
	}
	shortCode := out[0].ShortURL[strings.LastIndex(out[0].ShortURL, "/")+1:]
	getReq := httptest.NewRequest(http.MethodGet, "/"+shortCode, nil)
	getRR := httptest.NewRecorder()
	r.ServeHTTP(getRR, getReq)
	if getRR.Code != http.StatusTemporaryRedirect || getRR.Header().Get("Location") != "https://example.com/batch-a" {
		t.Errorf("GET /%s: code=%d location=%q", shortCode, getRR.Code, getRR.Header().Get("Location"))
	}
}

func TestRouter_POST_ApiShortenBatch_EmptyRejected(t *testing.T) {
	r := routerFromMain(t, "localhost:8080")
	req := httptest.NewRequest(http.MethodPost, "/api/shorten/batch", bytes.NewReader([]byte("[]")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("POST /api/shorten/batch empty: got status %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

// Тесты Gzip middleware через роутер (как в main).

func TestRouter_Gzip_AcceptEncoding_ReturnsCompressed(t *testing.T) {
	r := routerFromMain(t, "localhost:8080")

	body, _ := json.Marshal(map[string]string{"url": "https://example.com/page"})
	req := httptest.NewRequest(http.MethodPost, "/api/shorten", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept-Encoding", "gzip")
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("POST /api/shorten: got status %d, want %d", rr.Code, http.StatusCreated)
	}
	if enc := rr.Header().Get("Content-Encoding"); enc != "gzip" {
		t.Errorf("Content-Encoding = %q, want gzip", enc)
	}

	gr, err := gzip.NewReader(rr.Body)
	if err != nil {
		t.Fatalf("response body is not gzip: %v", err)
	}
	defer gr.Close()
	decoded, err := io.ReadAll(gr)
	if err != nil {
		t.Fatalf("gzip read: %v", err)
	}
	var out struct {
		Result string `json:"result"`
	}
	if err := json.Unmarshal(decoded, &out); err != nil {
		t.Fatalf("decoded JSON: %v", err)
	}
	if !strings.HasPrefix(out.Result, "http://localhost:8080/") {
		t.Errorf("response url %q does not start with base URL", out.Result)
	}
}

func TestRouter_Gzip_NoAcceptEncoding_ReturnsUncompressed(t *testing.T) {
	r := routerFromMain(t, "localhost:8080")

	body, _ := json.Marshal(map[string]string{"url": "https://example.com/page"})
	req := httptest.NewRequest(http.MethodPost, "/api/shorten", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("POST /api/shorten: got status %d, want %d", rr.Code, http.StatusCreated)
	}
	if enc := rr.Header().Get("Content-Encoding"); enc != "" {
		t.Errorf("Content-Encoding = %q, want empty (no compression)", enc)
	}
	var out struct {
		Result string `json:"result"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&out); err != nil {
		t.Fatalf("response JSON: %v", err)
	}
	if !strings.HasPrefix(out.Result, "http://localhost:8080/") {
		t.Errorf("response url %q does not start with base URL", out.Result)
	}
}
