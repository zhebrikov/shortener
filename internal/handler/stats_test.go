package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zhebrikov/shortener/internal/storage"
)

func TestInternalStats_forbiddenWhenSubnetEmpty(t *testing.T) {
	store := storage.NewMemoryStorage()
	h := InternalStats("", store)

	req := httptest.NewRequest(http.MethodGet, "/api/internal/stats", nil)
	req.Header.Set("X-Real-IP", "127.0.0.1")
	rr := httptest.NewRecorder()
	h(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusForbidden)
	}
}

func TestInternalStats_forbiddenWhenIPOutsideSubnet(t *testing.T) {
	store := storage.NewMemoryStorage()
	h := InternalStats("10.0.0.0/8", store)

	req := httptest.NewRequest(http.MethodGet, "/api/internal/stats", nil)
	req.Header.Set("X-Real-IP", "192.168.1.1")
	rr := httptest.NewRecorder()
	h(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusForbidden)
	}
}

func TestInternalStats_success(t *testing.T) {
	store := storage.NewMemoryStorage()
	_ = store.WriteStorage(storage.Link{ShortURL: "http://localhost/a", OriginalURL: "https://a.example", UserID: "user-1"})
	_ = store.WriteStorage(storage.Link{ShortURL: "http://localhost/b", OriginalURL: "https://b.example", UserID: "user-2"})
	_ = store.WriteStorage(storage.Link{ShortURL: "http://localhost/c", OriginalURL: "https://c.example", UserID: "user-1"})

	h := InternalStats("127.0.0.0/8", store)
	req := httptest.NewRequest(http.MethodGet, "/api/internal/stats", nil)
	req.Header.Set("X-Real-IP", "127.0.0.1")
	rr := httptest.NewRecorder()
	h(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	var out internalStatsResponse
	if err := json.NewDecoder(rr.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.URLs != 3 || out.Users != 2 {
		t.Fatalf("response = %+v, want urls=3 users=2", out)
	}
}
