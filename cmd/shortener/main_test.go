package main

import (
	"bytes"
	"compress/gzip"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-chi/chi/v5"
	"github.com/golang-migrate/migrate/v4"
	"github.com/zhebrikov/shortener/internal/asyncdelete"
	"github.com/zhebrikov/shortener/internal/audit"
	"github.com/zhebrikov/shortener/internal/auth"
	appconfig "github.com/zhebrikov/shortener/internal/config"
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
	return handler.NewShortenerHandler(shortener, store, asyncdelete.NewWorker(store), nil)
}

// routerFromMain возвращает роутер в том же виде, что и в main (для интеграционных тестов).
func routerFromMain(t *testing.T, baseURL string) http.Handler {
	t.Helper()
	shortener := service.NewShortener(baseURL)
	store := storageFromTestFile(t)
	h := handler.NewShortenerHandler(shortener, store, asyncdelete.NewWorker(store), nil)
	r := chi.NewRouter()
	r.Use(logger.Middleware(zap.NewNop()))
	r.Use(middleware.Gzip)
	r.Use(auth.Middleware("test-secret-key-for-router-tests"))
	r.Post("/", h.CreateLink)
	r.Get("/{shortCode}", h.GetLink)
	r.Post("/api/shorten", h.CreateLinkJSON)
	r.Post("/api/shorten/batch", h.CreateLinkBatch)
	r.Get("/api/user/urls", func(w http.ResponseWriter, r *http.Request) {
		handler.ListUserURLs(w, r, store)
	})
	r.Delete("/api/user/urls", h.DeleteUserURLs)
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

	t.Run("AUDIT_FILE and AUDIT_URL from env", func(t *testing.T) {
		oldAF, afOk := saveEnv("AUDIT_FILE")
		oldAU, auOk := saveEnv("AUDIT_URL")
		os.Setenv("AUDIT_FILE", "/tmp/audit.log")
		os.Setenv("AUDIT_URL", "https://audit.example/hook")
		defer restoreEnv("AUDIT_FILE", oldAF, afOk)
		defer restoreEnv("AUDIT_URL", oldAU, auOk)

		got, err := getConfig(defaultCfg)
		if err != nil {
			t.Fatalf("getConfig() unexpected error: %v", err)
		}
		if got.AuditFile != "/tmp/audit.log" {
			t.Errorf("getConfig() AuditFile = %q; want /tmp/audit.log", got.AuditFile)
		}
		if got.AuditURL != "https://audit.example/hook" {
			t.Errorf("getConfig() AuditURL = %q; want https://audit.example/hook", got.AuditURL)
		}
	})

	t.Run("missing AUDIT_FILE uses default from flags", func(t *testing.T) {
		oldAF, afOk := saveEnv("AUDIT_FILE")
		oldAU, auOk := saveEnv("AUDIT_URL")
		os.Unsetenv("AUDIT_FILE")
		os.Unsetenv("AUDIT_URL")
		defer restoreEnv("AUDIT_FILE", oldAF, afOk)
		defer restoreEnv("AUDIT_URL", oldAU, auOk)

		got, err := getConfig(Config{
			ServerAddress: "localhost:8080",
			BaseURL:       "http://example.com",
			FileStorage:   "",
			DatabaseDsn:   "",
			AuditFile:     "/flags/audit.log",
			AuditURL:      "https://flags.example/h",
		})
		if err != nil {
			t.Fatalf("getConfig: %v", err)
		}
		if got.AuditFile != "/flags/audit.log" {
			t.Errorf("AuditFile = %q", got.AuditFile)
		}
		if got.AuditURL != "https://flags.example/h" {
			t.Errorf("AuditURL = %q", got.AuditURL)
		}
	})

	t.Run("ENABLE_HTTPS from env", func(t *testing.T) {
		oldHTTPS, httpsOk := saveEnv("ENABLE_HTTPS")
		os.Setenv("ENABLE_HTTPS", "true")
		defer restoreEnv("ENABLE_HTTPS", oldHTTPS, httpsOk)

		got, err := getConfig(defaultCfg)
		if err != nil {
			t.Fatalf("getConfig() unexpected error: %v", err)
		}
		if !got.EnableHTTPS {
			t.Errorf("getConfig() EnableHTTPS = false; want true")
		}
	})

	t.Run("missing ENABLE_HTTPS uses default from flags", func(t *testing.T) {
		oldHTTPS, httpsOk := saveEnv("ENABLE_HTTPS")
		os.Unsetenv("ENABLE_HTTPS")
		defer restoreEnv("ENABLE_HTTPS", oldHTTPS, httpsOk)

		got, err := getConfig(Config{
			ServerAddress: "localhost:8080",
			BaseURL:       "http://example.com",
			EnableHTTPS:   true,
		})
		if err != nil {
			t.Fatalf("getConfig: %v", err)
		}
		if !got.EnableHTTPS {
			t.Error("EnableHTTPS = false; want true from defaults")
		}
	})

	t.Run("invalid ENABLE_HTTPS returns error", func(t *testing.T) {
		oldHTTPS, httpsOk := saveEnv("ENABLE_HTTPS")
		os.Setenv("ENABLE_HTTPS", "not-a-bool")
		defer restoreEnv("ENABLE_HTTPS", oldHTTPS, httpsOk)

		if _, err := getConfig(defaultCfg); err == nil {
			t.Fatal("expected error for invalid ENABLE_HTTPS")
		}
	})
}

func TestApplyFileConfig(t *testing.T) {
	enableHTTPS := true
	fileCfg := appconfig.File{
		ServerAddress: strPtr("file:9090"),
		BaseURL:       strPtr("http://file.example"),
		EnableHTTPS:   &enableHTTPS,
	}
	got := applyFileConfig(defaultConfig(), fileCfg)
	if got.ServerAddress != "file:9090" {
		t.Errorf("ServerAddress = %q; want file:9090", got.ServerAddress)
	}
	if got.BaseURL != "http://file.example" {
		t.Errorf("BaseURL = %q; want http://file.example", got.BaseURL)
	}
	if !got.EnableHTTPS {
		t.Error("EnableHTTPS = false; want true")
	}
}

func TestApplyVisitedFlags(t *testing.T) {
	cfg := Config{
		ServerAddress: "file:9090",
		BaseURL:       "http://file.example",
		EnableHTTPS:   true,
	}
	visited := map[string]bool{"a": true, "s": true}
	flagCfg := Config{
		ServerAddress: "flag:8080",
		BaseURL:       "http://file.example",
		EnableHTTPS:   false,
	}
	got := applyVisitedFlags(cfg, visited, flagCfg)
	if got.ServerAddress != "flag:8080" {
		t.Errorf("ServerAddress = %q; want flag:8080", got.ServerAddress)
	}
	if got.BaseURL != "http://file.example" {
		t.Errorf("BaseURL = %q; want value from file when flag not visited", got.BaseURL)
	}
	if got.EnableHTTPS {
		t.Error("EnableHTTPS = true; want false from visited flag")
	}
}

func TestRun_configFile(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")
	content := `{
		"server_address": "localhost:9091",
		"base_url": "http://config.example",
		"enable_https": true
	}`
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	oldListenTLS := appListenTLS
	tlsCalled := false
	appListenTLS = func(addr string, _ http.Handler) error {
		tlsCalled = true
		if addr != ":9091" {
			t.Errorf("listen addr = %q; want :9091", addr)
		}
		return nil
	}
	t.Cleanup(func() { appListenTLS = oldListenTLS })

	err := run([]string{"-c", configPath}, func(string, http.Handler) error {
		t.Fatal("HTTP listen must not be called when config enables HTTPS")
		return nil
	})
	if err != nil {
		t.Fatalf("run() err = %v", err)
	}
	if !tlsCalled {
		t.Fatal("expected appListenTLS to be called from config file")
	}
}

func TestRun_configFileFlagOverridesFile(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")
	content := `{"server_address": "localhost:9091"}`
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	var listenAddr string
	err := run([]string{"-c", configPath, "-a", "localhost:7070"}, func(addr string, _ http.Handler) error {
		listenAddr = addr
		return nil
	})
	if err != nil {
		t.Fatalf("run() err = %v", err)
	}
	if listenAddr != ":7070" {
		t.Errorf("listen addr = %q; want :7070", listenAddr)
	}
}

func TestRun_configFileEnvOverridesFileAndFlag(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")
	content := `{"server_address": "localhost:9091"}`
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	oldAddr, addrOk := os.LookupEnv("SERVER_ADDRESS")
	os.Setenv("SERVER_ADDRESS", "localhost:6060")
	t.Cleanup(func() {
		if addrOk {
			os.Setenv("SERVER_ADDRESS", oldAddr)
		} else {
			os.Unsetenv("SERVER_ADDRESS")
		}
	})

	var listenAddr string
	err := run([]string{"-c", configPath, "-a", "localhost:7070"}, func(addr string, _ http.Handler) error {
		listenAddr = addr
		return nil
	})
	if err != nil {
		t.Fatalf("run() err = %v", err)
	}
	if listenAddr != ":6060" {
		t.Errorf("listen addr = %q; want :6060 from env", listenAddr)
	}
}

func TestRun_configPathFromEnv(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")
	content := `{"server_address": "localhost:8088"}`
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	oldConfig, configOk := os.LookupEnv("CONFIG")
	os.Setenv("CONFIG", configPath)
	t.Cleanup(func() {
		if configOk {
			os.Setenv("CONFIG", oldConfig)
		} else {
			os.Unsetenv("CONFIG")
		}
	})

	var listenAddr string
	err := run(nil, func(addr string, _ http.Handler) error {
		listenAddr = addr
		return nil
	})
	if err != nil {
		t.Fatalf("run() err = %v", err)
	}
	if listenAddr != ":8088" {
		t.Errorf("listen addr = %q; want :8088", listenAddr)
	}
}

func TestRun_configFileError(t *testing.T) {
	if err := run([]string{"-c", "/nonexistent/config.json"}, func(string, http.Handler) error { return nil }); err == nil {
		t.Fatal("expected config file error")
	}
}

func strPtr(s string) *string {
	return &s
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

	body, err := json.Marshal(map[string]string{"url": "https://example.com/page"})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
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

	body, err := json.Marshal(map[string]string{"url": "https://example.com"})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
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

	body, err := json.Marshal(map[string]string{"url": "https://example.com/page"})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
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

	body, err := json.Marshal(map[string]string{"url": "https://example.com/from-json"})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
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

	body, err := json.Marshal([]map[string]string{
		{"correlation_id": "id1", "original_url": "https://example.com/batch-a"},
		{"correlation_id": "id2", "original_url": "https://example.com/batch-b"},
	})
	if err != nil {
		t.Fatalf("marshal batch body: %v", err)
	}
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

	body, err := json.Marshal(map[string]string{"url": "https://example.com/page"})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
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

func TestRouter_GET_UserURLs_EmptyCookie_Unauthorized(t *testing.T) {
	r := routerFromMain(t, "localhost:8080")

	req := httptest.NewRequest(http.MethodGet, "/api/user/urls", nil)
	req.Header.Set("Cookie", auth.CookieName+"=")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("GET /api/user/urls с пустой кукой: статус %d, ожидалось %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestRouter_GET_UserURLs_NoLinks_NoContent(t *testing.T) {
	r := routerFromMain(t, "localhost:8080")

	req := httptest.NewRequest(http.MethodGet, "/api/user/urls", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("GET /api/user/urls без ссылок: статус %d, ожидалось %d", rr.Code, http.StatusNoContent)
	}
}

func TestRouter_GET_UserURLs_AfterJSONShorten_SameSession(t *testing.T) {
	r := routerFromMain(t, "localhost:8080")

	body, err := json.Marshal(map[string]string{"url": "https://example.com/mine"})
	if err != nil {
		t.Fatal(err)
	}
	postReq := httptest.NewRequest(http.MethodPost, "/api/shorten", bytes.NewReader(body))
	postReq.Header.Set("Content-Type", "application/json")
	postRR := httptest.NewRecorder()
	r.ServeHTTP(postRR, postReq)
	if postRR.Code != http.StatusCreated {
		t.Fatalf("POST /api/shorten: статус %d", postRR.Code)
	}

	postResp := postRR.Result()
	defer postResp.Body.Close()

	getReq := httptest.NewRequest(http.MethodGet, "/api/user/urls", nil)
	for _, c := range postResp.Cookies() {
		getReq.AddCookie(c)
	}
	getRR := httptest.NewRecorder()
	r.ServeHTTP(getRR, getReq)

	if getRR.Code != http.StatusOK {
		t.Fatalf("GET /api/user/urls: статус %d", getRR.Code)
	}
	var items []handler.UserURLItem
	if err := json.NewDecoder(getRR.Body).Decode(&items); err != nil {
		t.Fatalf("JSON: %v", err)
	}
	if len(items) != 1 || items[0].OriginalURL != "https://example.com/mine" {
		t.Errorf("ответ = %+v", items)
	}
}

func TestRouter_Gzip_NoAcceptEncoding_ReturnsUncompressed(t *testing.T) {
	r := routerFromMain(t, "localhost:8080")

	body, err := json.Marshal(map[string]string{"url": "https://example.com/page"})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
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

func TestRouter_auditFileAfterPOST(t *testing.T) {
	auditPath := filepath.Join(t.TempDir(), "audit.log")
	pub := audit.NewPublisher(audit.NewFileObserver(auditPath))

	shortener := service.NewShortener("http://localhost:8080")
	store := storageFromTestFile(t)
	h := handler.NewShortenerHandler(shortener, store, asyncdelete.NewWorker(store), pub)

	r := chi.NewRouter()
	r.Use(logger.Middleware(zap.NewNop()))
	r.Use(auth.Middleware("test-secret-key-for-router-tests"))
	r.Post("/", h.CreateLink)
	r.Get("/{shortCode}", h.GetLink)
	r.Post("/api/shorten", h.CreateLinkJSON)

	wantURL := "https://router-audit.example/r"
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte(wantURL)))
	req.Header.Set("Content-Type", "text/plain")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("POST /: status %d", rr.Code)
	}

	deadline := time.Now().Add(2 * time.Second)
	var data []byte
	var err error
	for time.Now().Before(deadline) {
		data, err = os.ReadFile(auditPath)
		if err == nil && len(data) > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("read audit file: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("audit file empty")
	}

	var ev struct {
		Action string `json:"action"`
		URL    string `json:"url"`
	}
	line := bytes.TrimSpace(data)
	if err := json.Unmarshal(line, &ev); err != nil {
		t.Fatalf("audit JSON: %v data=%q", err, data)
	}
	if ev.Action != "shorten" || ev.URL != wantURL {
		t.Errorf("audit event = %+v, want shorten / %q", ev, wantURL)
	}
}

func TestRouter_auditHTTPPostSink(t *testing.T) {
	var mu sync.Mutex
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method %s", r.Method)
		}
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		gotBody = b
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	pub := audit.NewPublisher(audit.NewHTTPObserver(srv.URL, srv.Client()))
	shortener := service.NewShortener("http://localhost:8080")
	store := storageFromTestFile(t)
	h := handler.NewShortenerHandler(shortener, store, asyncdelete.NewWorker(store), pub)

	r := chi.NewRouter()
	r.Use(logger.Middleware(zap.NewNop()))
	r.Use(auth.Middleware("test-secret-key-for-router-tests"))
	r.Post("/", h.CreateLink)

	wantURL := "https://router-http-audit.example/x"
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte(wantURL)))
	req.Header.Set("Content-Type", "text/plain")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("POST /: status %d", rr.Code)
	}

	deadline := time.Now().Add(2 * time.Second)
	var body []byte
	for time.Now().Before(deadline) {
		mu.Lock()
		if len(gotBody) > 0 {
			body = append([]byte(nil), gotBody...)
		}
		mu.Unlock()
		if len(body) > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if len(body) == 0 {
		t.Fatal("audit server received no body")
	}
	var ev struct {
		Action string `json:"action"`
		URL    string `json:"url"`
	}
	if err := json.Unmarshal(body, &ev); err != nil {
		t.Fatalf("audit JSON: %v", err)
	}
	if ev.Action != "shorten" || ev.URL != wantURL {
		t.Errorf("got %+v", ev)
	}
}

func TestNewApp_memoryStore(t *testing.T) {
	dir := t.TempDir()
	auditPath := filepath.Join(dir, "audit.log")
	application, err := newApp(Config{
		ServerAddress: "localhost:8081",
		BaseURL:       "localhost:8081",
		AuditFile:     auditPath,
	})
	if err != nil {
		t.Fatal(err)
	}
	if application.Handler == nil || application.Port != ":8081" {
		t.Fatalf("app = %+v", application)
	}

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("https://example.com/new-app"))
	req.Header.Set("Content-Type", "text/plain")
	rr := httptest.NewRecorder()
	application.Handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("POST / status = %d", rr.Code)
	}
}

func TestNewApp_fileStore(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "links.json")
	application, err := newApp(Config{
		ServerAddress: "localhost:8082",
		BaseURL:       "localhost:8082",
		FileStorage:   filePath,
	})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	rr := httptest.NewRecorder()
	application.Handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /ping status = %d", rr.Code)
	}
}

func TestNewApp_invalidServerAddress(t *testing.T) {
	if _, err := newApp(Config{ServerAddress: "invalid-no-port", BaseURL: "localhost:8080"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestRun_flagParseError(t *testing.T) {
	err := run([]string{"-unknown-flag"}, func(string, http.Handler) error { return nil })
	if err == nil {
		t.Fatal("expected flag parse error")
	}
}

func TestRun_memoryStore(t *testing.T) {
	err := run(nil, func(_ string, _ http.Handler) error {
		return nil
	})
	if err != nil {
		t.Fatalf("run() err = %v", err)
	}
}

func TestRun_invalidAddress(t *testing.T) {
	if err := run([]string{"-a", "bad-host"}, func(string, http.Handler) error { return nil }); err == nil {
		t.Fatal("expected error")
	}
}

func TestRun_databaseConnectionError(t *testing.T) {
	err := run([]string{"-d", "postgres://invalid:5432/nodb"}, func(string, http.Handler) error { return nil })
	if err == nil {
		t.Fatal("expected connection error")
	}
}

func TestExitCode_success(t *testing.T) {
	if exitCode(nil, func(string, http.Handler) error { return nil }) != 0 {
		t.Fatal("expected exit code 0")
	}
}

func TestExitCode_error(t *testing.T) {
	if exitCode([]string{"-a", "invalid-no-port"}, func(string, http.Handler) error { return nil }) == 0 {
		t.Fatal("expected non-zero exit code")
	}
}

func TestMain_callsExit(t *testing.T) {
	var code int
	oldExit := osExit
	oldListen := appListen
	osExit = func(c int) { code = c }
	appListen = func(string, http.Handler) error { return nil }
	defer func() {
		osExit = oldExit
		appListen = oldListen
	}()

	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"shortener-test"}
	main()
	if code != 0 {
		t.Fatalf("main() exit code = %d, want 0", code)
	}
}

func TestNewApp_postgresStore(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	oldConnect := dbConnect
	oldMigrate := runMigrate
	dbConnect = func(string) (*sql.DB, error) { return db, nil }
	runMigrate = func(string, string) error { return nil }
	defer func() {
		dbConnect = oldConnect
		runMigrate = oldMigrate
	}()

	application, err := newApp(Config{
		ServerAddress: "localhost:8084",
		BaseURL:       "localhost:8084",
		DatabaseDsn:   "postgres://mock",
	})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	rr := httptest.NewRecorder()
	application.Handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /ping status = %d", rr.Code)
	}
}

func TestNewApp_dbConnectError(t *testing.T) {
	oldConnect := dbConnect
	dbConnect = func(string) (*sql.DB, error) { return nil, os.ErrInvalid }
	defer func() { dbConnect = oldConnect }()

	if _, err := newApp(Config{
		ServerAddress: "localhost:8085",
		BaseURL:       "localhost:8085",
		DatabaseDsn:   "postgres://fail",
	}); err == nil {
		t.Fatal("expected db connect error")
	}
}

func TestNewApp_migrationError(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	oldConnect := dbConnect
	oldMigrate := runMigrate
	dbConnect = func(string) (*sql.DB, error) { return db, nil }
	runMigrate = func(string, string) error { return os.ErrInvalid }
	defer func() {
		dbConnect = oldConnect
		runMigrate = oldMigrate
	}()

	if _, err := newApp(Config{
		ServerAddress: "localhost:8086",
		BaseURL:       "localhost:8086",
		DatabaseDsn:   "postgres://mock",
	}); err == nil {
		t.Fatal("expected migration error")
	}
}

func TestRunMigrations_invalidPath(t *testing.T) {
	if err := runMigrations("/nonexistent-migrations-dir-xyz", "postgres://localhost/db"); err == nil {
		t.Fatal("expected migration error")
	}
}

func TestRunMigrations_moduleMigrationsDir(t *testing.T) {
	migrations := filepath.Join("..", "..", "migrations")
	if _, err := os.Stat(migrations); err != nil {
		t.Skip("migrations directory not found from cmd/shortener")
	}
	// migrate.New должен открыть file://; Up упадёт без живой БД — покрываем ветку успешного New.
	if err := runMigrations(migrations, "postgres://127.0.0.1:1/nodb?connect_timeout=1"); err == nil {
		t.Fatal("expected migration up error without database")
	}
}

func TestNewApp_auditObservers(t *testing.T) {
	dir := t.TempDir()
	auditPath := filepath.Join(dir, "audit.log")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	application, err := newApp(Config{
		ServerAddress: "localhost:8083",
		BaseURL:       "localhost:8083",
		AuditFile:     auditPath,
		AuditURL:      srv.URL,
		SecretKey:     "",
	})
	if err != nil {
		t.Fatal(err)
	}
	if application.Handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestBuildInfoOrNA(t *testing.T) {
	if got := buildInfoOrNA(""); got != "N/A" {
		t.Errorf("empty: got %q, want N/A", got)
	}
	if got := buildInfoOrNA("v1.0.0"); got != "v1.0.0" {
		t.Errorf("value: got %q, want v1.0.0", got)
	}
}

func TestPrintBuildInfo(t *testing.T) {
	oldVersion, oldDate, oldCommit := buildVersion, buildDate, buildCommit
	t.Cleanup(func() {
		buildVersion = oldVersion
		buildDate = oldDate
		buildCommit = oldCommit
	})

	t.Run("N/A when empty", func(t *testing.T) {
		buildVersion, buildDate, buildCommit = "", "", ""
		out := captureStdout(t, printBuildInfo)
		want := "Build version: N/A\nBuild date: N/A\nBuild commit: N/A\n"
		if out != want {
			t.Errorf("stdout:\n%q\nwant:\n%q", out, want)
		}
	})

	t.Run("prints ldflags values", func(t *testing.T) {
		buildVersion = "1.2.3"
		buildDate = "2024-01-15"
		buildCommit = "abc123"
		out := captureStdout(t, printBuildInfo)
		want := "Build version: 1.2.3\nBuild date: 2024-01-15\nBuild commit: abc123\n"
		if out != want {
			t.Errorf("stdout:\n%q\nwant:\n%q", out, want)
		}
	})
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	t.Cleanup(func() {
		os.Stdout = old
		_ = r.Close()
		_ = w.Close()
	})

	done := make(chan struct{})
	var buf bytes.Buffer
	go func() {
		_, _ = io.Copy(&buf, r)
		close(done)
	}()

	fn()
	_ = w.Close()
	<-done
	return buf.String()
}

func TestRun_getConfigError(t *testing.T) {
	old := appGetConfig
	appGetConfig = func(Config) (Config, error) {
		return Config{}, errors.New("config error")
	}
	t.Cleanup(func() { appGetConfig = old })

	if err := run(nil, func(string, http.Handler) error { return nil }); err == nil {
		t.Fatal("expected config error")
	}
}

func TestRun_listenError(t *testing.T) {
	want := errors.New("listen failed")
	err := run(nil, func(string, http.Handler) error { return want })
	if !errors.Is(err, want) {
		t.Fatalf("run() err = %v, want %v", err, want)
	}
}

func TestRun_enableHTTPS(t *testing.T) {
	oldListenTLS := appListenTLS
	tlsCalled := false
	appListenTLS = func(string, http.Handler) error {
		tlsCalled = true
		return nil
	}
	t.Cleanup(func() { appListenTLS = oldListenTLS })

	err := run([]string{"-s"}, func(string, http.Handler) error {
		t.Fatal("HTTP listen must not be called when HTTPS is enabled")
		return nil
	})
	if err != nil {
		t.Fatalf("run() err = %v", err)
	}
	if !tlsCalled {
		t.Fatal("expected appListenTLS to be called")
	}
}

func TestRun_enableHTTPSFromEnv(t *testing.T) {
	oldListenTLS := appListenTLS
	oldHTTPS, httpsOk := os.LookupEnv("ENABLE_HTTPS")
	os.Setenv("ENABLE_HTTPS", "true")
	tlsCalled := false
	appListenTLS = func(string, http.Handler) error {
		tlsCalled = true
		return nil
	}
	t.Cleanup(func() {
		appListenTLS = oldListenTLS
		if httpsOk {
			os.Setenv("ENABLE_HTTPS", oldHTTPS)
		} else {
			os.Unsetenv("ENABLE_HTTPS")
		}
	})

	err := run(nil, func(string, http.Handler) error {
		t.Fatal("HTTP listen must not be called when ENABLE_HTTPS is set")
		return nil
	})
	if err != nil {
		t.Fatalf("run() err = %v", err)
	}
	if !tlsCalled {
		t.Fatal("expected appListenTLS to be called")
	}
}

func TestNewApp_loggerError(t *testing.T) {
	old := appLoggerNew
	appLoggerNew = func(string) (*zap.Logger, error) {
		return nil, errors.New("logger init failed")
	}
	t.Cleanup(func() { appLoggerNew = old })

	if _, err := newApp(Config{
		ServerAddress: "localhost:8090",
		BaseURL:       "localhost:8090",
	}); err == nil {
		t.Fatal("expected logger error")
	}
}

func TestNewApp_listUserURLs(t *testing.T) {
	application, err := newApp(Config{
		ServerAddress: "localhost:8091",
		BaseURL:       "localhost:8091",
		SecretKey:     "list-user-urls-secret",
	})
	if err != nil {
		t.Fatal(err)
	}

	body, err := json.Marshal(map[string]string{"url": "https://example.com/list-user"})
	if err != nil {
		t.Fatal(err)
	}
	postReq := httptest.NewRequest(http.MethodPost, "/api/shorten", bytes.NewReader(body))
	postReq.Header.Set("Content-Type", "application/json")
	postRR := httptest.NewRecorder()
	application.Handler.ServeHTTP(postRR, postReq)
	if postRR.Code != http.StatusCreated {
		t.Fatalf("POST /api/shorten: status %d", postRR.Code)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/user/urls", nil)
	for _, c := range postRR.Result().Cookies() {
		getReq.AddCookie(c)
	}
	getRR := httptest.NewRecorder()
	application.Handler.ServeHTTP(getRR, getReq)

	if getRR.Code != http.StatusOK {
		t.Fatalf("GET /api/user/urls: status %d", getRR.Code)
	}
	var items []handler.UserURLItem
	if err := json.NewDecoder(getRR.Body).Decode(&items); err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].OriginalURL != "https://example.com/list-user" {
		t.Errorf("items = %+v", items)
	}
}

func TestNewApp_postgresDefaultMigrationPath(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	var gotMigrationPath string
	oldConnect := dbConnect
	oldMigrate := runMigrate
	dbConnect = func(string) (*sql.DB, error) { return db, nil }
	runMigrate = func(path, dsn string) error {
		gotMigrationPath = path
		return nil
	}
	t.Cleanup(func() {
		dbConnect = oldConnect
		runMigrate = oldMigrate
	})

	if _, err := newApp(Config{
		ServerAddress: "localhost:8092",
		BaseURL:       "localhost:8092",
		DatabaseDsn:   "postgres://mock",
		MigrationPath: "",
	}); err != nil {
		t.Fatal(err)
	}
	if gotMigrationPath != "migrations" {
		t.Errorf("migration path = %q, want migrations", gotMigrationPath)
	}
}

func TestRunMigrations_success(t *testing.T) {
	oldNew, oldUp, oldClose := migrateNew, migrateUp, migrateClose
	m := &migrate.Migrate{}
	var upCalled bool
	migrateNew = func(string, string) (*migrate.Migrate, error) { return m, nil }
	migrateUp = func(got *migrate.Migrate) error {
		upCalled = true
		if got != m {
			t.Errorf("migrateUp called with %p, want %p", got, m)
		}
		return migrate.ErrNoChange
	}
	migrateClose = func(*migrate.Migrate) {}
	t.Cleanup(func() {
		migrateNew = oldNew
		migrateUp = oldUp
		migrateClose = oldClose
	})

	if err := runMigrations("migrations", "postgres://unused"); err != nil {
		t.Fatalf("runMigrations() err = %v", err)
	}
	if !upCalled {
		t.Fatal("migrateUp was not called")
	}
}

func TestRunMigrations_fallbackFromCmdDir(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	repoMigrations := filepath.Join(wd, "..", "..", "migrations")
	if _, err := os.Stat(repoMigrations); err != nil {
		t.Skip("migrations directory not found")
	}
	cmdDir := filepath.Join(wd, "..")
	if err := os.Chdir(cmdDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })

	err = runMigrations("__invalid__", "postgres://127.0.0.1:1/nodb?connect_timeout=1")
	if err == nil {
		t.Fatal("expected migration up error without database")
	}
}

func TestRunMigrations_upReturnsError(t *testing.T) {
	oldNew, oldUp, oldClose := migrateNew, migrateUp, migrateClose
	migrateNew = func(string, string) (*migrate.Migrate, error) { return &migrate.Migrate{}, nil }
	migrateUp = func(*migrate.Migrate) error { return fmt.Errorf("migration up failed") }
	migrateClose = func(*migrate.Migrate) {}
	t.Cleanup(func() {
		migrateNew = oldNew
		migrateUp = oldUp
		migrateClose = oldClose
	})

	if err := runMigrations("migrations", "postgres://unused"); err == nil {
		t.Fatal("expected migration up error")
	}
}
