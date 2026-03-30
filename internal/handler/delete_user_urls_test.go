package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/zhebrikov/shortener/internal/asyncdelete"
	"github.com/zhebrikov/shortener/internal/auth"
	"github.com/zhebrikov/shortener/internal/service"
)

func TestDeleteUserURLs_Unauthorized(t *testing.T) {
	store := mustTempStorage(t, "[]")
	wkr := asyncdelete.NewWorker(store)
	req := httptest.NewRequest(http.MethodDelete, "/api/user/urls", bytes.NewReader([]byte(`["a"]`)))
	rr := httptest.NewRecorder()

	DeleteUserURLs(rr, req, wkr)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("статус = %d, ожидалось %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestDeleteUserURLs_Unauthorized_EmptyCookie(t *testing.T) {
	store := mustTempStorage(t, "[]")
	wkr := asyncdelete.NewWorker(store)
	req := httptest.NewRequest(http.MethodDelete, "/api/user/urls", bytes.NewReader([]byte(`["a"]`)))
	req = req.WithContext(auth.WithEmptyAuthCookie(context.Background()))
	rr := httptest.NewRecorder()

	DeleteUserURLs(rr, req, wkr)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("статус = %d, ожидалось %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestDeleteUserURLs_BadJSON(t *testing.T) {
	store := mustTempStorage(t, "[]")
	wkr := asyncdelete.NewWorker(store)
	req := httptest.NewRequest(http.MethodDelete, "/api/user/urls", bytes.NewReader([]byte(`not json`)))
	req = req.WithContext(auth.WithUserID(context.Background(), "user-1"))
	rr := httptest.NewRecorder()

	DeleteUserURLs(rr, req, wkr)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("статус = %d, ожидалось %d", rr.Code, http.StatusBadRequest)
	}
}

func TestDeleteUserURLs_Accepted_AndSoftDelete(t *testing.T) {
	const uid = "user-abc"
	const shortCode = "abcd1234"
	const orig = "https://example.com/x"
	data := `[{"uuid":1,"short_url":"http://localhost:8080/` + shortCode + `","original_url":"` + orig + `","user_id":"` + uid + `"}]`
	store := mustTempStorage(t, data)
	wkr := asyncdelete.NewWorker(store)

	body, _ := json.Marshal([]string{shortCode})
	req := httptest.NewRequest(http.MethodDelete, "/api/user/urls", bytes.NewReader(body))
	req = req.WithContext(auth.WithUserID(context.Background(), uid))
	rr := httptest.NewRecorder()

	DeleteUserURLs(rr, req, wkr)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("статус = %d, ожидалось %d", rr.Code, http.StatusAccepted)
	}

	shortener := service.NewShortener("localhost:8080")
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		u, gone := shortener.GetLink(shortCode, store)
		if gone {
			if u != nil {
				t.Fatal("gone with non-nil url")
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("ожидалось мягкое удаление и gone=true")
}
