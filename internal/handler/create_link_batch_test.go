package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zhebrikov/shortener/internal/asyncdelete"
	"github.com/zhebrikov/shortener/internal/auth"
	"github.com/zhebrikov/shortener/internal/service"
	"github.com/zhebrikov/shortener/internal/storage"
)

func TestCreateLinkBatch_success(t *testing.T) {
	shortener := service.NewShortener("localhost:8080")
	store := storage.NewMemoryStorage()
	body, _ := json.Marshal([]BatchRequestItem{
		{CorrelationID: "1", OriginalURL: "https://example.com/a"},
		{CorrelationID: "2", OriginalURL: "https://example.com/a"},
		{CorrelationID: "3", OriginalURL: "https://example.com/b"},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/shorten/batch", bytes.NewReader(body))
	req = req.WithContext(auth.WithUserID(req.Context(), "u1"))
	rr := httptest.NewRecorder()

	CreateLinkBatch(rr, req, shortener, store)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	var out []BatchResponseItem
	if err := json.NewDecoder(rr.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if len(out) != 3 || out[0].ShortURL != out[1].ShortURL {
		t.Fatalf("response = %+v", out)
	}
}

func TestCreateLinkBatch_validationErrors(t *testing.T) {
	shortener := service.NewShortener("localhost:8080")
	store := storage.NewMemoryStorage()

	tests := []struct {
		name string
		body string
	}{
		{"empty array", "[]"},
		{"invalid json", `{`},
		{"empty original_url", `[{"correlation_id":"1","original_url":"  "}]`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/shorten/batch", strings.NewReader(tt.body))
			rr := httptest.NewRecorder()
			CreateLinkBatch(rr, req, shortener, store)
			if rr.Code != http.StatusBadRequest {
				t.Fatalf("status = %d", rr.Code)
			}
		})
	}
}

func TestCreateLinkBatch_readBodyError(t *testing.T) {
	shortener := service.NewShortener("localhost:8080")
	req := httptest.NewRequest(http.MethodPost, "/api/shorten/batch", errReader{})
	rr := httptest.NewRecorder()
	CreateLinkBatch(rr, req, shortener, storage.NewMemoryStorage())
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestCreateLinkBatch_duplicateURL(t *testing.T) {
	shortener := service.NewShortener("localhost:8080")
	base := storage.NewMemoryStorage()
	_ = base.WriteStorage(storage.Link{UUID: 1, ShortURL: "http://localhost/existing", OriginalURL: "https://dup.com"})
	store := stubStore{LinkStore: base}

	body, _ := json.Marshal([]BatchRequestItem{{CorrelationID: "1", OriginalURL: "https://dup.com"}})
	req := httptest.NewRequest(http.MethodPost, "/api/shorten/batch", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	CreateLinkBatch(rr, req, shortener, store)
	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
}

func TestCreateLinkBatch_storeErrors(t *testing.T) {
	shortener := service.NewShortener("localhost:8080")
	body, _ := json.Marshal([]BatchRequestItem{{CorrelationID: "1", OriginalURL: "https://example.com/x"}})

	t.Run("write error", func(t *testing.T) {
		store := badShortenerStore{MemoryStorage: storage.NewMemoryStorage()}
		req := httptest.NewRequest(http.MethodPost, "/api/shorten/batch", bytes.NewReader(body))
		rr := httptest.NewRecorder()
		CreateLinkBatch(rr, req, shortener, store)
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d", rr.Code)
		}
	})

	t.Run("duplicate get error", func(t *testing.T) {
		base := storage.NewMemoryStorage()
		_ = base.WriteStorage(storage.Link{UUID: 1, ShortURL: "http://localhost/x", OriginalURL: "https://example.com/x"})
		store := stubStore{LinkStore: base, getShortURLErr: errors.New("lookup failed")}
		req := httptest.NewRequest(http.MethodPost, "/api/shorten/batch", bytes.NewReader(body))
		rr := httptest.NewRecorder()
		CreateLinkBatch(rr, req, shortener, store)
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d", rr.Code)
		}
	})
}

func TestCreateLinkBatch_invalidBaseURL(t *testing.T) {
	shortener := service.NewShortener("http://%zz")
	body, _ := json.Marshal([]BatchRequestItem{{CorrelationID: "1", OriginalURL: "https://example.com/x"}})
	req := httptest.NewRequest(http.MethodPost, "/api/shorten/batch", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	CreateLinkBatch(rr, req, shortener, storage.NewMemoryStorage())
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestShortenerHandler_wrappers(t *testing.T) {
	shortener := service.NewShortener("localhost:8080")
	store := storage.NewMemoryStorage()
	h := NewShortenerHandler(shortener, store, asyncdelete.NewWorker(store), nil)

	t.Run("CreateLinkJSON", func(t *testing.T) {
		body, _ := json.Marshal(Input{URL: "https://example.com/wrap"})
		req := httptest.NewRequest(http.MethodPost, "/api/shorten", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		h.CreateLinkJSON(rr, req)
		if rr.Code != http.StatusCreated {
			t.Fatalf("status = %d", rr.Code)
		}
	})

	t.Run("CreateLinkBatch", func(t *testing.T) {
		body, _ := json.Marshal([]BatchRequestItem{{CorrelationID: "1", OriginalURL: "https://example.com/batch-wrap"}})
		req := httptest.NewRequest(http.MethodPost, "/api/shorten/batch", bytes.NewReader(body))
		rr := httptest.NewRecorder()
		h.CreateLinkBatch(rr, req)
		if rr.Code != http.StatusCreated {
			t.Fatalf("status = %d", rr.Code)
		}
	})

	t.Run("DeleteUserURLs", func(t *testing.T) {
		body := `["abc12345"]`
		req := httptest.NewRequest(http.MethodDelete, "/api/user/urls", strings.NewReader(body))
		req = req.WithContext(auth.WithUserID(req.Context(), "u1"))
		rr := httptest.NewRecorder()
		h.DeleteUserURLs(rr, req)
		if rr.Code != http.StatusAccepted {
			t.Fatalf("status = %d", rr.Code)
		}
	})
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }
func (errReader) Close() error             { return nil }
