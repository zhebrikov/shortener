package handler

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/zhebrikov/shortener/internal/audit"
	"github.com/zhebrikov/shortener/internal/auth"
	"github.com/zhebrikov/shortener/internal/service"
)

// auditChanObserver реализует audit.Observer для тестов (асинхронный Publish).
type auditChanObserver struct {
	ch chan audit.Event
}

func (o *auditChanObserver) OnAudit(_ context.Context, ev audit.Event) error {
	o.ch <- ev
	return nil
}

func newTestAuditPublisher(buf int) (*audit.Publisher, <-chan audit.Event) {
	ch := make(chan audit.Event, buf)
	pub := audit.NewPublisher(&auditChanObserver{ch: ch})
	return pub, ch
}

func readOneAudit(t *testing.T, ch <-chan audit.Event) audit.Event {
	t.Helper()
	select {
	case ev := <-ch:
		return ev
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for audit event")
		panic("unreachable")
	}
}

func expectNoAudit(t *testing.T, ch <-chan audit.Event, wait time.Duration) {
	t.Helper()
	select {
	case ev := <-ch:
		t.Fatalf("unexpected audit event: %+v", ev)
	case <-time.After(wait):
	}
}

func TestCreateLink_auditAfter201(t *testing.T) {
	shortener := service.NewShortener("localhost:8080")
	store := mustTempStorage(t, "[]")
	pub, ch := newTestAuditPublisher(4)

	body := []byte("https://example.com/audit-plain")
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "text/plain")
	req = req.WithContext(auth.WithUserID(req.Context(), "user-xyz"))

	rr := httptest.NewRecorder()
	CreateLink(rr, req, shortener, store, pub)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status %d", rr.Code)
	}
	ev := readOneAudit(t, ch)
	if ev.Action != audit.ActionShorten {
		t.Errorf("action = %q, want %q", ev.Action, audit.ActionShorten)
	}
	if ev.URL != string(body) {
		t.Errorf("url = %q", ev.URL)
	}
	if ev.UserID != "user-xyz" {
		t.Errorf("user_id = %q", ev.UserID)
	}
	if ev.Ts <= 0 {
		t.Errorf("ts = %d", ev.Ts)
	}
}

func TestCreateLink_auditNotOn409Duplicate(t *testing.T) {
	shortener := service.NewShortener("localhost:8080")
	store := mustTempStorage(t, "[]")
	pub, ch := newTestAuditPublisher(4)

	body := []byte("https://dup.example/a")
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "text/plain")

	CreateLink(httptest.NewRecorder(), req, shortener, store, pub)
	readOneAudit(t, ch)

	req2 := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req2.Header.Set("Content-Type", "text/plain")
	rr2 := httptest.NewRecorder()
	CreateLink(rr2, req2, shortener, store, pub)

	if rr2.Code != http.StatusConflict {
		t.Fatalf("second POST: status %d, want 409", rr2.Code)
	}
	expectNoAudit(t, ch, 300*time.Millisecond)
}

func TestCreateLinkJSON_auditAfter201(t *testing.T) {
	shortener := service.NewShortener("localhost:8080")
	store := mustTempStorage(t, "[]")
	pub, ch := newTestAuditPublisher(4)

	payload := mustMarshal(t, Input{URL: "https://json.example/p"})
	req := httptest.NewRequest(http.MethodPost, "/api/shorten", bytes.NewReader(payload))
	req = req.WithContext(auth.WithUserID(req.Context(), "uid-json"))

	rr := httptest.NewRecorder()
	CreateLinkJSON(rr, req, shortener, store, pub)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status %d", rr.Code)
	}
	ev := readOneAudit(t, ch)
	if ev.Action != audit.ActionShorten || ev.URL != "https://json.example/p" || ev.UserID != "uid-json" {
		t.Errorf("event %+v", ev)
	}
}

func TestCreateLinkJSON_auditNotOn409Duplicate(t *testing.T) {
	shortener := service.NewShortener("localhost:8080")
	store := mustTempStorage(t, "[]")
	pub, ch := newTestAuditPublisher(4)

	url := "https://dup-json.example/x"
	payload := mustMarshal(t, Input{URL: url})

	CreateLinkJSON(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/api/shorten", bytes.NewReader(payload)), shortener, store, pub)
	readOneAudit(t, ch)

	rr2 := httptest.NewRecorder()
	CreateLinkJSON(rr2, httptest.NewRequest(http.MethodPost, "/api/shorten", bytes.NewReader(payload)), shortener, store, pub)
	if rr2.Code != http.StatusConflict {
		t.Fatalf("status %d", rr2.Code)
	}
	expectNoAudit(t, ch, 300*time.Millisecond)
}

func TestGetLink_auditOn307(t *testing.T) {
	shortener := service.NewShortener("localhost:8080")
	originalURL := "https://follow.example/page"
	shortURL, _ := shortener.CreateLink(originalURL)
	shortCode := shortURL[strings.LastIndex(shortURL, "/")+1:]

	storageJSON := `[{"uuid":1,"short_url":"` + shortCode + `","original_url":"` + originalURL + `"}]`
	store := mustTempStorage(t, storageJSON)
	pub, ch := newTestAuditPublisher(4)

	req := httptest.NewRequest(http.MethodGet, "/"+shortCode, nil)
	req = req.WithContext(auth.WithUserID(req.Context(), "follower"))

	rr := httptest.NewRecorder()
	GetLink(rr, req, shortener, store, pub)

	if rr.Code != http.StatusTemporaryRedirect {
		t.Fatalf("status %d", rr.Code)
	}
	ev := readOneAudit(t, ch)
	if ev.Action != audit.ActionFollow || ev.URL != originalURL || ev.UserID != "follower" {
		t.Errorf("event %+v", ev)
	}
}

func TestGetLink_auditNotOn404(t *testing.T) {
	shortener := service.NewShortener("localhost:8080")
	store := mustTempStorage(t, "[]")
	pub, ch := newTestAuditPublisher(2)

	req := httptest.NewRequest(http.MethodGet, "/nope", nil)
	GetLink(httptest.NewRecorder(), req, shortener, store, pub)
	expectNoAudit(t, ch, 300*time.Millisecond)
}

func TestGetLink_auditNotOn410(t *testing.T) {
	shortener := service.NewShortener("localhost:8080")
	originalURL := "https://gone.example/x"
	shortURL, _ := shortener.CreateLink(originalURL)
	shortCode := shortURL[strings.LastIndex(shortURL, "/")+1:]
	storageJSON := `[{"uuid":1,"short_url":"` + shortCode + `","original_url":"` + originalURL + `","is_deleted":true}]`
	store := mustTempStorage(t, storageJSON)
	pub, ch := newTestAuditPublisher(2)

	req := httptest.NewRequest(http.MethodGet, "/"+shortCode, nil)
	GetLink(httptest.NewRecorder(), req, shortener, store, pub)
	expectNoAudit(t, ch, 300*time.Millisecond)
}

func TestShortenerHandler_passesAuditToHandlers(t *testing.T) {
	shortener := service.NewShortener("localhost:8080")
	store := mustTempStorage(t, "[]")
	pub, ch := newTestAuditPublisher(8)
	h := NewShortenerHandler(shortener, store, nil, pub)

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte("https://via-handler.example/")))
	req.Header.Set("Content-Type", "text/plain")
	h.CreateLink(httptest.NewRecorder(), req)

	ev := readOneAudit(t, ch)
	if ev.Action != audit.ActionShorten || ev.URL != "https://via-handler.example/" {
		t.Fatalf("bad event %+v", ev)
	}
}
