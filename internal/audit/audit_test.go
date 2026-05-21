package audit

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestNewPublisher_emptyReturnsNil(t *testing.T) {
	if NewPublisher() != nil {
		t.Fatal("NewPublisher() without observers must return nil")
	}
}

func TestPublisher_nilSafe(t *testing.T) {
	var p *Publisher
	p.Publish(Event{Ts: 1, Action: ActionShorten, URL: "https://a"})
}

func TestFileObserver_OnAudit_appendsSecondLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.log")
	obs := NewFileObserver(path)
	if err := obs.OnAudit(context.Background(), Event{Ts: 1, Action: ActionShorten, URL: "https://a"}); err != nil {
		t.Fatal(err)
	}
	if err := obs.OnAudit(context.Background(), Event{Ts: 2, Action: ActionFollow, URL: "https://b"}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("want 2 lines, got %d: %q", len(lines), data)
	}
	var e1, e2 Event
	if err := json.Unmarshal([]byte(lines[0]), &e1); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(lines[1]), &e2); err != nil {
		t.Fatal(err)
	}
	if e1.URL != "https://a" || e2.URL != "https://b" {
		t.Errorf("events %+v %+v", e1, e2)
	}
}

func TestFileObserver_OnAudit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.log")
	obs := NewFileObserver(path)
	ev := Event{Ts: 42, Action: ActionShorten, UserID: "u1", URL: "https://example.com/x"}
	if err := obs.OnAudit(context.Background(), ev); err != nil {
		t.Fatalf("OnAudit: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var got Event
	if err := json.Unmarshal(data[:len(data)-1], &got); err != nil {
		t.Fatalf("json: %v", err)
	}
	if data[len(data)-1] != '\n' {
		t.Errorf("want trailing newline")
	}
	if got.Ts != 42 || got.Action != ActionShorten || got.UserID != "u1" || got.URL != ev.URL {
		t.Errorf("got %+v, want %+v", got, ev)
	}
}

func TestHTTPObserver_OnAudit(t *testing.T) {
	var mu sync.Mutex
	var received Event
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", ct)
		}
		var ev Event
		if err := json.NewDecoder(r.Body).Decode(&ev); err != nil {
			t.Errorf("decode: %v", err)
		}
		mu.Lock()
		received = ev
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	obs := NewHTTPObserver(srv.URL, srv.Client())
	ev := Event{Ts: 7, Action: ActionFollow, URL: "https://orig"}
	if err := obs.OnAudit(context.Background(), ev); err != nil {
		t.Fatalf("OnAudit: %v", err)
	}
	mu.Lock()
	got := received
	mu.Unlock()
	if got.URL != ev.URL || got.Action != ActionFollow {
		t.Errorf("server got %+v", got)
	}
}

func TestPublisher_notifiesObservers(t *testing.T) {
	ch := make(chan Event, 2)
	o1 := chanObserver{ch: ch}
	o2 := chanObserver{ch: ch}
	p := NewPublisher(o1, o2)
	p.Publish(Event{Ts: 1, Action: ActionShorten, URL: "https://z"})
	for i := 0; i < 2; i++ {
		select {
		case <-ch:
		case <-time.After(2 * time.Second):
			t.Fatalf("timeout waiting observer %d", i)
		}
	}
}

type chanObserver struct {
	ch chan Event
}

func (c chanObserver) OnAudit(_ context.Context, ev Event) error {
	c.ch <- ev
	return nil
}

type errObserver struct{}

func (errObserver) OnAudit(context.Context, Event) error {
	return os.ErrInvalid
}

func TestPublisher_observerErrorDoesNotPanic(t *testing.T) {
	p := NewPublisher(errObserver{})
	p.Publish(Event{Ts: 1, Action: ActionShorten, URL: "https://x"})
	time.Sleep(50 * time.Millisecond)
}

func TestFileObserver_OnAudit_invalidPath(t *testing.T) {
	dir := t.TempDir()
	obs := NewFileObserver(dir)
	if err := obs.OnAudit(context.Background(), Event{Ts: 1, Action: ActionShorten, URL: "https://a"}); err == nil {
		t.Fatal("expected error when path is a directory")
	}
}

func TestHTTPObserver_OnAudit_requestError(t *testing.T) {
	obs := NewHTTPObserver("://bad-url", http.DefaultClient)
	if err := obs.OnAudit(context.Background(), Event{Ts: 1, Action: ActionShorten, URL: "https://a"}); err == nil {
		t.Fatal("expected error for invalid URL")
	}
}

type failingAuditWriter struct{}

func (failingAuditWriter) Write([]byte) (int, error) { return 0, os.ErrInvalid }
func (failingAuditWriter) Close() error              { return nil }

func TestFileObserver_OnAudit_writeError(t *testing.T) {
	old := auditAppender
	auditAppender = func(string) (io.WriteCloser, error) {
		return failingAuditWriter{}, nil
	}
	defer func() { auditAppender = old }()

	obs := NewFileObserver(filepath.Join(t.TempDir(), "audit.log"))
	if err := obs.OnAudit(context.Background(), Event{Ts: 1, Action: ActionShorten, URL: "https://a.com"}); err == nil {
		t.Fatal("expected write error")
	}
}

func TestFileObserver_OnAudit_closeError(t *testing.T) {
	old := auditAppender
	auditAppender = func(string) (io.WriteCloser, error) {
		return closeErrWriter{}, nil
	}
	defer func() { auditAppender = old }()

	obs := NewFileObserver(filepath.Join(t.TempDir(), "audit.log"))
	if err := obs.OnAudit(context.Background(), Event{Ts: 1, Action: ActionShorten, URL: "https://a.com"}); err == nil {
		t.Fatal("expected close error")
	}
}

type closeErrWriter struct{}

func (closeErrWriter) Write(p []byte) (int, error) { return len(p), nil }
func (closeErrWriter) Close() error                { return os.ErrInvalid }

func TestFileObserver_OnAudit_readOnlyFileWriteError(t *testing.T) {
	old := auditAppender
	auditAppender = func(string) (io.WriteCloser, error) {
		return nil, os.ErrPermission
	}
	defer func() { auditAppender = old }()

	obs := NewFileObserver(filepath.Join(t.TempDir(), "readonly.log"))
	err := obs.OnAudit(context.Background(), Event{Ts: 1, Action: ActionShorten, URL: "https://a.com"})
	if err == nil {
		t.Fatal("expected error when audit file is not writable")
	}
}

func TestHTTPObserver_OnAudit_networkError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.Close()
	obs := NewHTTPObserver(srv.URL, srv.Client())
	if err := obs.OnAudit(context.Background(), Event{Ts: 1, Action: ActionShorten, URL: "https://a"}); err == nil {
		t.Fatal("expected error for closed server")
	}
}
