package logger

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestNew_validLevel(t *testing.T) {
	log, err := New("info")
	if err != nil || log == nil {
		t.Fatalf("New(info) = %v, %v", log, err)
	}
	_ = log.Sync()
}

func TestNew_invalidLevel(t *testing.T) {
	if _, err := New("not-a-level"); err == nil {
		t.Fatal("expected error for invalid level")
	}
}

func TestMiddleware_logsRequest(t *testing.T) {
	core, recorded := observer.New(zap.InfoLevel)
	log := zap.New(core)

	h := Middleware(log)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest(http.MethodPost, "/test?x=1", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d", rr.Code)
	}
	if recorded.Len() != 1 {
		t.Fatalf("logs = %d, want 1", recorded.Len())
	}
	entry := recorded.All()[0]
	if entry.Message != "request completed" {
		t.Fatalf("message = %q", entry.Message)
	}
}

func TestMiddleware_defaultStatusOnWriteOnly(t *testing.T) {
	core, recorded := observer.New(zap.InfoLevel)
	log := zap.New(core)

	h := Middleware(log)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("body"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	entry := recorded.All()[0]
	for _, f := range entry.Context {
		if f.Key == "status" && f.Integer == int64(http.StatusOK) {
			return
		}
	}
	t.Fatalf("expected status 200 in log: %+v", entry.Context)
}

func TestRequestLogger(t *testing.T) {
	core, recorded := observer.New(zap.DebugLevel)
	log := zap.New(core)

	called := false
	h := RequestLogger(log, func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	req := httptest.NewRequest(http.MethodGet, "/path", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if !called {
		t.Fatal("handler not called")
	}
	if recorded.Len() != 1 || recorded.All()[0].Message != "got incoming HTTP request" {
		t.Fatalf("logs = %+v", recorded.All())
	}
}
