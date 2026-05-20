package handler

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zhebrikov/shortener/internal/service"
	"github.com/zhebrikov/shortener/internal/storage"
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
			name:           "без Content-Type — 400",
			method:         http.MethodPost,
			body:           []byte("https://example.com"),
			storage:        "[]",
			wantStatus:     http.StatusBadRequest,
			wantBodySubstr: "text/plain",
		},
		{
			name:           "пустое тело — 400",
			method:         http.MethodPost,
			body:           nil,
			contentType:    "text/plain",
			storage:        "[]",
			wantStatus:     http.StatusBadRequest,
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

			CreateLink(rr, req, shortener, store, nil)

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

func TestCreateLink_duplicateConflict(t *testing.T) {
	shortener := service.NewShortener("localhost:8080")
	store := mustTempStorage(t, `[{"uuid":1,"short_url":"http://localhost:8080/exist","original_url":"https://example.com/dup"}]`)
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("https://example.com/dup"))
	req.Header.Set("Content-Type", "text/plain")
	rr := httptest.NewRecorder()
	CreateLink(rr, req, shortener, store, nil)
	if rr.Code != http.StatusConflict {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
}

func TestCreateLink_storeErrors(t *testing.T) {
	shortener := service.NewShortener("localhost:8080")

	t.Run("NextLinkUUID error", func(t *testing.T) {
		store := stubStore{LinkStore: mustTempStorage(t, "[]"), nextUUIDErr: errors.New("uuid failed")}
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("https://example.com/x"))
		req.Header.Set("Content-Type", "text/plain")
		rr := httptest.NewRecorder()
		CreateLink(rr, req, shortener, store, nil)
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d", rr.Code)
		}
	})

	t.Run("WriteStorage error", func(t *testing.T) {
		store := stubStore{LinkStore: mustTempStorage(t, "[]"), writeErr: errors.New("write failed")}
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("https://example.com/y"))
		req.Header.Set("Content-Type", "text/plain")
		rr := httptest.NewRecorder()
		CreateLink(rr, req, shortener, store, nil)
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d", rr.Code)
		}
	})

	t.Run("duplicate lookup error", func(t *testing.T) {
		base := mustTempStorage(t, `[{"uuid":1,"short_url":"http://localhost/8080/x","original_url":"https://example.com/dup2"}]`)
		store := stubStore{LinkStore: base, writeErr: storage.ErrDuplicateURL, getShortURLErr: errors.New("lookup failed")}
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("https://example.com/dup2"))
		req.Header.Set("Content-Type", "text/plain")
		rr := httptest.NewRecorder()
		CreateLink(rr, req, shortener, store, nil)
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d", rr.Code)
		}
	})

	t.Run("shortener error", func(t *testing.T) {
		badShortener := service.NewShortener("http://%zz")
		store := mustTempStorage(t, "[]")
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("https://example.com/z"))
		req.Header.Set("Content-Type", "text/plain")
		rr := httptest.NewRecorder()
		CreateLink(rr, req, badShortener, store, nil)
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d", rr.Code)
		}
	})
}
