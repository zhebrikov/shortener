package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zhebrikov/shortener/internal/auth"
)

func TestListUserURLs_Unauthorized_NoUserInContext(t *testing.T) {
	store := mustTempStorage(t, "[]")
	req := httptest.NewRequest(http.MethodGet, "/api/user/urls", nil)
	rr := httptest.NewRecorder()

	ListUserURLs(rr, req, store)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("ListUserURLs: статус = %d, ожидалось %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestListUserURLs_Unauthorized_EmptyCookie(t *testing.T) {
	store := mustTempStorage(t, "[]")
	req := httptest.NewRequest(http.MethodGet, "/api/user/urls", nil)
	req = req.WithContext(auth.WithEmptyAuthCookie(context.Background()))
	rr := httptest.NewRecorder()

	ListUserURLs(rr, req, store)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("ListUserURLs: статус = %d, ожидалось %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestListUserURLs_NoContent_EmptyList(t *testing.T) {
	store := mustTempStorage(t, "[]")
	req := httptest.NewRequest(http.MethodGet, "/api/user/urls", nil)
	req = req.WithContext(auth.WithUserID(context.Background(), "user-1"))
	rr := httptest.NewRecorder()

	ListUserURLs(rr, req, store)

	if rr.Code != http.StatusNoContent {
		t.Errorf("ListUserURLs: статус = %d, ожидалось %d", rr.Code, http.StatusNoContent)
	}
}

func TestListUserURLs_OK_ReturnsUserLinks(t *testing.T) {
	const uid = "user-abc"
	data := `[{"uuid":1,"short_url":"http://localhost:8080/abc","original_url":"https://example.com/x","user_id":"` + uid + `"}]`
	store := mustTempStorage(t, data)
	req := httptest.NewRequest(http.MethodGet, "/api/user/urls", nil)
	req = req.WithContext(auth.WithUserID(context.Background(), uid))
	rr := httptest.NewRecorder()

	ListUserURLs(rr, req, store)

	if rr.Code != http.StatusOK {
		t.Fatalf("ListUserURLs: статус = %d, ожидалось %d", rr.Code, http.StatusOK)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	var out []UserURLItem
	if err := json.NewDecoder(rr.Body).Decode(&out); err != nil {
		t.Fatalf("JSON: %v", err)
	}
	if len(out) != 1 || out[0].OriginalURL != "https://example.com/x" {
		t.Errorf("ответ = %+v", out)
	}
}

type failEncodeResponseWriter struct {
	http.ResponseWriter
	headerWritten bool
}

func (w *failEncodeResponseWriter) WriteHeader(code int) {
	w.headerWritten = true
	w.ResponseWriter.WriteHeader(code)
}

func (w *failEncodeResponseWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}

func TestListUserURLs_encodeError(t *testing.T) {
	const uid = "user-encode"
	data := `[{"uuid":1,"short_url":"http://localhost/x","original_url":"https://example.com/x","user_id":"` + uid + `"}]`
	store := mustTempStorage(t, data)
	req := httptest.NewRequest(http.MethodGet, "/api/user/urls", nil)
	req = req.WithContext(auth.WithUserID(context.Background(), uid))
	rr := httptest.NewRecorder()
	ListUserURLs(&failEncodeResponseWriter{ResponseWriter: rr}, req, store)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestListUserURLs_storeError(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/user/urls", nil)
	req = req.WithContext(auth.WithUserID(context.Background(), "user-1"))
	rr := httptest.NewRecorder()
	ListUserURLs(rr, req, errReadStore{err: errors.New("store down")})
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", rr.Code)
	}
}
