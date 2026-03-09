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

func TestCreateLinkJSON(t *testing.T) {
	shortener := service.NewShortener("localhost:8080")

	tests := []struct {
		name           string
		method         string
		body           []byte
		contentType    string
		storage        string
		wantStatus     int
		wantResultPref string
		wantBodySubstr string
	}{
		{
			name:           "успех — 201, application/json, result с короткой ссылкой",
			method:         http.MethodPost,
			body:           mustMarshal(t, Input{URL: "https://example.com/page"}),
			contentType:    "application/json",
			storage:        "[]",
			wantStatus:     http.StatusCreated,
			wantResultPref: "http://localhost:8080/",
		},
		{
			name:        "не JSON — 400",
			method:      http.MethodPost,
			body:        []byte("not json"),
			contentType: "application/json",
			storage:     "[]",
			wantStatus:  http.StatusBadRequest,
			wantBodySubstr: "Invalid request body",
		},
		{
			name:        "битый JSON — 400",
			method:      http.MethodPost,
			body:        []byte(`{"url": }`),
			contentType: "application/json",
			storage:     "[]",
			wantStatus:  http.StatusBadRequest,
		},
		{
			name:           "пустой url — 201, возвращает короткую ссылку",
			method:         http.MethodPost,
			body:           mustMarshal(t, Input{URL: ""}),
			contentType:    "application/json",
			storage:        "[]",
			wantStatus:     http.StatusCreated,
			wantResultPref: "http://localhost:8080/",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := mustTempStorage(t, tt.storage)
			req := httptest.NewRequest(tt.method, "/api/shorten", bytes.NewReader(tt.body))
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}
			rr := httptest.NewRecorder()

			CreateLinkJSON(rr, req, shortener, store)

			if rr.Code != tt.wantStatus {
				t.Errorf("CreateLinkJSON: статус = %d, ожидалось %d", rr.Code, tt.wantStatus)
			}
			if tt.wantResultPref != "" {
				var out Output
				if err := json.NewDecoder(rr.Body).Decode(&out); err != nil {
					t.Fatalf("CreateLinkJSON: невалидный JSON: %v", err)
				}
				if !strings.HasPrefix(out.Result, tt.wantResultPref) {
					t.Errorf("CreateLinkJSON: result %q не начинается с %q", out.Result, tt.wantResultPref)
				}
			}
			if tt.wantBodySubstr != "" {
				if body := rr.Body.String(); !strings.Contains(body, tt.wantBodySubstr) {
					t.Errorf("CreateLinkJSON: в теле ответа нет %q", tt.wantBodySubstr)
				}
			}
		})
	}
}

func mustMarshal(t *testing.T, v interface{}) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
