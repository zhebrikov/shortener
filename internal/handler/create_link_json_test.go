package handler

import (
	"bytes"
	"encoding/json"
	"errors"
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
			name:           "не JSON — 400",
			method:         http.MethodPost,
			body:           []byte("not json"),
			contentType:    "application/json",
			storage:        "[]",
			wantStatus:     http.StatusBadRequest,
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

			CreateLinkJSON(rr, req, shortener, store, nil)

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

func TestCreateLinkJSON_storeErrors(t *testing.T) {
	shortener := service.NewShortener("localhost:8080")

	t.Run("NextLinkUUID error", func(t *testing.T) {
		store := stubStore{LinkStore: mustTempStorage(t, "[]"), nextUUIDErr: errors.New("uuid failed")}
		req := httptest.NewRequest(http.MethodPost, "/api/shorten", bytes.NewReader(mustMarshal(t, Input{URL: "https://example.com/a"})))
		rr := httptest.NewRecorder()
		CreateLinkJSON(rr, req, shortener, store, nil)
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d", rr.Code)
		}
	})

	t.Run("duplicate conflict", func(t *testing.T) {
		store := mustTempStorage(t, `[{"uuid":1,"short_url":"http://localhost/8080/ex","original_url":"https://example.com/dup-json"}]`)
		req := httptest.NewRequest(http.MethodPost, "/api/shorten", bytes.NewReader(mustMarshal(t, Input{URL: "https://example.com/dup-json"})))
		rr := httptest.NewRecorder()
		CreateLinkJSON(rr, req, shortener, store, nil)
		if rr.Code != http.StatusConflict {
			t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
		}
	})

	t.Run("write error", func(t *testing.T) {
		store := stubStore{LinkStore: mustTempStorage(t, "[]"), writeErr: errors.New("write failed")}
		req := httptest.NewRequest(http.MethodPost, "/api/shorten", bytes.NewReader(mustMarshal(t, Input{URL: "https://example.com/write-err"})))
		rr := httptest.NewRecorder()
		CreateLinkJSON(rr, req, shortener, store, nil)
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d", rr.Code)
		}
	})

	t.Run("duplicate lookup error", func(t *testing.T) {
		base := mustTempStorage(t, `[{"uuid":1,"short_url":"http://localhost/ex","original_url":"https://example.com/dup-json2"}]`)
		store := stubStore{LinkStore: base, getShortURLErr: errors.New("lookup failed")}
		req := httptest.NewRequest(http.MethodPost, "/api/shorten", bytes.NewReader(mustMarshal(t, Input{URL: "https://example.com/dup-json2"})))
		rr := httptest.NewRecorder()
		CreateLinkJSON(rr, req, shortener, store, nil)
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d", rr.Code)
		}
	})

	t.Run("read body error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/shorten", errReader{})
		rr := httptest.NewRecorder()
		CreateLinkJSON(rr, req, shortener, mustTempStorage(t, "[]"), nil)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d", rr.Code)
		}
	})

	t.Run("shortener error", func(t *testing.T) {
		badShortener := service.NewShortener("http://%")
		req := httptest.NewRequest(http.MethodPost, "/api/shorten", bytes.NewReader(mustMarshal(t, Input{URL: "https://example.com/a"})))
		rr := httptest.NewRecorder()
		CreateLinkJSON(rr, req, badShortener, mustTempStorage(t, "[]"), nil)
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
		}
	})
}
