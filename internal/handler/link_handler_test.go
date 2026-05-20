package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zhebrikov/shortener/internal/service"
)

func TestNewShortenerHandler(t *testing.T) {
	shortener := service.NewShortener("test:9090")
	store := mustTempStorage(t, "[]")
	h := NewShortenerHandler(shortener, store, nil, nil)
	if h == nil {
		t.Fatal("NewShortenerHandler returned nil")
	}
}

func TestShortenerHandler_CreateLink(t *testing.T) {
	baseURL := "localhost:8080"
	shortener := service.NewShortener(baseURL)

	tests := []struct {
		name           string
		method         string
		body           []byte
		contentType    string
		storage        string
		wantStatus     int
		wantBodyPrefix string
	}{
		{
			name:           "POST с телом и text/plain — 201, возвращает короткую ссылку",
			method:         http.MethodPost,
			body:           []byte("https://example.com/page"),
			contentType:    "text/plain",
			storage:        "[]",
			wantStatus:     http.StatusCreated,
			wantBodyPrefix: "http://" + baseURL + "/",
		},
		{
			name:       "POST без Content-Type — 400",
			method:     http.MethodPost,
			body:       []byte("https://example.com"),
			storage:    "[]",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:        "POST с пустым телом — 400",
			method:      http.MethodPost,
			body:        nil,
			contentType: "text/plain",
			storage:     "[]",
			wantStatus:  http.StatusBadRequest,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := mustTempStorage(t, tt.storage)
			h := NewShortenerHandler(shortener, store, nil, nil)
			req := httptest.NewRequest(tt.method, "/", bytes.NewReader(tt.body))
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}
			rr := httptest.NewRecorder()

			h.CreateLink(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("CreateLink: статус = %d, ожидалось %d", rr.Code, tt.wantStatus)
			}
			if tt.wantBodyPrefix != "" {
				respBody := strings.TrimSpace(rr.Body.String())
				if !strings.HasPrefix(respBody, tt.wantBodyPrefix) {
					t.Errorf("CreateLink: ответ %q не начинается с %q", respBody, tt.wantBodyPrefix)
				}
			}
		})
	}
}

func TestShortenerHandler_GetLink(t *testing.T) {
	shortener := service.NewShortener("localhost:8080")
	originalURL := "https://example.com/redirect"
	shortURL, _ := shortener.CreateLink(originalURL)
	shortCode := shortURL[strings.LastIndex(shortURL, "/")+1:]

	tests := []struct {
		name       string
		method     string
		path       string
		storage    string
		wantStatus int
		wantLoc    string
	}{
		{
			name:       "GET по существующему коду — 307 и Location",
			method:     http.MethodGet,
			path:       "/" + shortCode,
			storage:    `[{"uuid":1,"short_url":"` + shortCode + `","original_url":"` + originalURL + `"}]`,
			wantStatus: http.StatusTemporaryRedirect,
			wantLoc:    originalURL,
		},
		{
			name:       "GET по неизвестному коду — 404",
			method:     http.MethodGet,
			path:       "/nonexistent",
			storage:    "[]",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "GET по удалённой ссылке — 410",
			method:     http.MethodGet,
			path:       "/" + shortCode,
			storage:    `[{"uuid":1,"short_url":"` + shortCode + `","original_url":"` + originalURL + `","is_deleted":true}]`,
			wantStatus: http.StatusGone,
		},
		{
			name:       "POST по коду — 405",
			method:     http.MethodPost,
			path:       "/" + shortCode,
			storage:    "[]",
			wantStatus: http.StatusMethodNotAllowed,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := mustTempStorage(t, tt.storage)
			h := NewShortenerHandler(shortener, store, nil, nil)
			req := httptest.NewRequest(tt.method, tt.path, nil)
			rr := httptest.NewRecorder()

			h.GetLink(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("GetLink: статус = %d, ожидалось %d", rr.Code, tt.wantStatus)
			}
			if tt.wantLoc != "" {
				if loc := rr.Header().Get("Location"); loc != tt.wantLoc {
					t.Errorf("GetLink: Location = %q, ожидалось %q", loc, tt.wantLoc)
				}
			}
		})
	}
}
