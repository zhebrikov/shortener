package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zhebrikov/shortener/internal/service"
)

func TestCreateLink(t *testing.T) {
	shortener := service.NewShortener("localhost:8080")

	tests := []struct {
		name           string
		method         string
		body           []byte
		contentType    string
		storage        string
		wantStatus     int
		wantBodyPrefix string
		wantBodySubstr string
	}{
		{
			name:           "успех — 201, text/plain, короткая ссылка",
			method:         http.MethodPost,
			body:           []byte("https://example.com/page"),
			contentType:    "text/plain",
			storage:        "[]",
			wantStatus:     http.StatusCreated,
			wantBodyPrefix: "http://localhost:8080/",
		},
		{
			name:        "без Content-Type — 400",
			method:      http.MethodPost,
			body:        []byte("https://example.com"),
			storage:     "[]",
			wantStatus:  http.StatusBadRequest,
			wantBodySubstr: "text/plain",
		},
		{
			name:        "пустое тело — 400",
			method:      http.MethodPost,
			body:        nil,
			contentType: "text/plain",
			storage:     "[]",
			wantStatus:  http.StatusBadRequest,
			wantBodySubstr: "Invalid request body",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := mustTempStorage(t, tt.storage)
			req := httptest.NewRequest(tt.method, "/", bytes.NewReader(tt.body))
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}
			rr := httptest.NewRecorder()

			CreateLink(rr, req, shortener, store)

			if rr.Code != tt.wantStatus {
				t.Errorf("CreateLink: статус = %d, ожидалось %d", rr.Code, tt.wantStatus)
			}
			if tt.wantBodyPrefix != "" {
				respBody := strings.TrimSpace(rr.Body.String())
				if !strings.HasPrefix(respBody, tt.wantBodyPrefix) {
					t.Errorf("CreateLink: ответ %q не начинается с %q", respBody, tt.wantBodyPrefix)
				}
			}
			if tt.wantBodySubstr != "" {
				if body := rr.Body.String(); !strings.Contains(body, tt.wantBodySubstr) {
					t.Errorf("CreateLink: в теле ответа нет %q", tt.wantBodySubstr)
				}
			}
		})
	}
}
