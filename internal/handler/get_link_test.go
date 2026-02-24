package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zhebrikov/shortener/internal/service"
)

func TestGetLink(t *testing.T) {
	shortener := service.NewShortener("localhost:8080")
	originalURL := "https://example.com/target"
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
			name:       "найден — 307 и Location",
			method:     http.MethodGet,
			path:       "/" + shortCode,
			storage:    `[{"uuid":1,"short_url":"` + shortCode + `","original_url":"` + originalURL + `"}]`,
			wantStatus: http.StatusTemporaryRedirect,
			wantLoc:    originalURL,
		},
		{
			name:       "не найден — 404",
			method:     http.MethodGet,
			path:       "/nonexistent123",
			storage:    "[]",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "корень / — 404",
			method:     http.MethodGet,
			path:       "/",
			storage:    "[]",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "POST — 405",
			method:     http.MethodPost,
			path:       "/" + shortCode,
			storage:    "[]",
			wantStatus: http.StatusMethodNotAllowed,
		},
		{
			name:       "PUT — 405",
			method:     http.MethodPut,
			path:       "/" + shortCode,
			storage:    "[]",
			wantStatus: http.StatusMethodNotAllowed,
		},
		{
			name:       "DELETE — 405",
			method:     http.MethodDelete,
			path:       "/" + shortCode,
			storage:    "[]",
			wantStatus: http.StatusMethodNotAllowed,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := mustTempStorage(t, tt.storage)
			req := httptest.NewRequest(tt.method, tt.path, nil)
			rr := httptest.NewRecorder()

			GetLink(rr, req, shortener, store)

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
