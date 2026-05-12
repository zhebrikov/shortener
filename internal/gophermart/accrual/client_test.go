package accrual

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestClientGetOrderOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/orders/1" {
			t.Fatalf("path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"order":"1","status":"PROCESSED","accrual":500}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, srv.Client())
	info, err := c.GetOrder("1")
	if err != nil {
		t.Fatal(err)
	}
	if info.Status != StatusProcessed || *info.Accrual != 500 {
		t.Fatalf("%+v", info)
	}
}

func TestClientGetOrder204(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	c := NewClient(srv.URL, srv.Client())
	_, err := c.GetOrder("x")
	if err != ErrNotRegistered {
		t.Fatalf("got %v", err)
	}
}

func TestClientGetOrder429(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "3")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte("slow down"))
	}))
	defer srv.Close()
	c := NewClient(srv.URL, srv.Client())
	_, err := c.GetOrder("x")
	var e *ErrTooManyRequests
	if !errors.As(err, &e) {
		t.Fatalf("got %v", err)
	}
	if e.RetryAfter != 3*time.Second {
		t.Fatalf("retry %v", e.RetryAfter)
	}
}

func TestParseRetryAfterDefault(t *testing.T) {
	if d := parseRetryAfter(""); d != time.Minute {
		t.Fatalf("%v", d)
	}
	if d := parseRetryAfter("not-a-number-or-date"); d != time.Minute {
		t.Fatalf("%v", d)
	}
}

func TestParseRetryAfterHTTPDate(t *testing.T) {
	future := time.Now().UTC().Add(2 * time.Second).Format(http.TimeFormat)
	if d := parseRetryAfter(future); d <= 0 {
		t.Fatalf("%v", d)
	}
	past := time.Now().UTC().Add(-2 * time.Hour).Format(http.TimeFormat)
	if d := parseRetryAfter(past); d != time.Minute {
		t.Fatalf("%v", d)
	}
}

func TestErrTooManyRequests_Error(t *testing.T) {
	e := &ErrTooManyRequests{RetryAfter: time.Second, Body: "x"}
	if !strings.Contains(e.Error(), "retry after") {
		t.Fatal(e.Error())
	}
}

func TestNewClientUsesDefaultHTTPClient(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	c := NewClient(srv.URL, nil)
	_, err := c.GetOrder("1")
	if err != ErrNotRegistered {
		t.Fatalf("%v", err)
	}
}

func TestClientGetOrder_badJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`not-json`))
	}))
	defer srv.Close()
	c := NewClient(srv.URL, srv.Client())
	_, err := c.GetOrder("1")
	if err == nil {
		t.Fatal("expected err")
	}
}

func TestClientGetOrder500(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	}))
	defer srv.Close()
	c := NewClient(srv.URL, srv.Client())
	_, err := c.GetOrder("1")
	if err == nil {
		t.Fatal("expected err")
	}
}

func TestClientGetOrderUnexpected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte("weird"))
	}))
	defer srv.Close()
	c := NewClient(srv.URL, srv.Client())
	_, err := c.GetOrder("1")
	if err == nil {
		t.Fatal("expected err")
	}
}

func TestClientGetOrderNetworkError(t *testing.T) {
	c := NewClient("http://127.0.0.1:1", &http.Client{Timeout: 10 * time.Millisecond})
	_, err := c.GetOrder("1")
	if err == nil {
		t.Fatal("expected err")
	}
}

func TestClientGetOrder_invalidURL(t *testing.T) {
	c := &Client{
		baseURL:    "http://\x00invalid",
		httpClient: http.DefaultClient,
	}
	_, err := c.GetOrder("1")
	if err == nil {
		t.Fatal("expected err")
	}
}
