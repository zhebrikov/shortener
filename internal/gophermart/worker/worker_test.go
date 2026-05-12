package worker

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/zhebrikov/shortener/internal/gophermart/accrual"
	"github.com/zhebrikov/shortener/internal/gophermart/store"
)

func TestMapAccrualResponse(t *testing.T) {
	v := 12.5
	cases := []struct {
		in *accrual.OrderInfo
		st string
		ac *float64
	}{
		{&accrual.OrderInfo{Status: accrual.StatusRegistered}, "PROCESSING", nil},
		{&accrual.OrderInfo{Status: accrual.StatusProcessing}, "PROCESSING", nil},
		{&accrual.OrderInfo{Status: accrual.StatusInvalid}, "INVALID", nil},
		{&accrual.OrderInfo{Status: accrual.StatusProcessed, Accrual: &v}, "PROCESSED", &v},
		{&accrual.OrderInfo{Status: accrual.OrderStatus("UNKNOWN")}, "PROCESSING", nil},
	}
	for _, tc := range cases {
		st, ac := mapAccrualResponse(tc.in)
		if st != tc.st {
			t.Fatalf("status %s vs %s for %+v", st, tc.st, tc.in)
		}
		if (ac == nil) != (tc.ac == nil) {
			t.Fatalf("accrual ptr mismatch %+v", tc.in)
		}
		if ac != nil && tc.ac != nil && *ac != *tc.ac {
			t.Fatalf("accrual value %v vs %v", *ac, *tc.ac)
		}
	}
}

type stubStore struct {
	jobs      []store.AccrualJob
	jobsErr   error
	syncCalls int
	syncErr   error
}

func (s *stubStore) PendingAccrualJobs(context.Context, int) ([]store.AccrualJob, error) {
	return s.jobs, s.jobsErr
}
func (s *stubStore) SyncAccrualStatus(ctx context.Context, orderID int64, status string, acc *float64) error {
	_ = ctx
	_ = orderID
	_ = status
	_ = acc
	s.syncCalls++
	return s.syncErr
}

func TestAccrualPoller_RunDefaults(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()
	p := &AccrualPoller{Store: &stubStore{}, Client: accrual.NewClient("http://127.0.0.1:9", nil)}
	p.Run(ctx)
}

func TestAccrualPoller_RunAppliesDefaultInterval(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()
	p := &AccrualPoller{Store: &stubStore{}, Client: accrual.NewClient("http://127.0.0.1:9", nil), Interval: 0, Batch: 0}
	p.Run(ctx)
}

func TestAccrualPoller_RunTickerBranch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(30 * time.Millisecond)
		cancel()
	}()
	p := &AccrualPoller{
		Store:    &stubStore{},
		Client:   accrual.NewClient(srv.URL, srv.Client()),
		Interval: time.Millisecond,
		Batch:    10,
	}
	p.Run(ctx)
}

func TestTick_jobsError(t *testing.T) {
	st := &stubStore{jobsErr: errors.New("db")}
	p := &AccrualPoller{Store: st, Client: accrual.NewClient("http://127.0.0.1:9", nil), Batch: 5}
	p.tick(context.Background())
}

func TestProcessJob_rateLimitedZeroRetry(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "0")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	st := &stubStore{}
	p := &AccrualPoller{
		Store:  st,
		Client: accrual.NewClient(srv.URL, srv.Client()),
		Sleep:  func(time.Duration) {},
	}
	err := p.processJob(context.Background(), store.AccrualJob{ID: 1, Number: "1"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestProcessJob_notRegistered(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	st := &stubStore{}
	p := &AccrualPoller{Store: st, Client: accrual.NewClient(srv.URL, srv.Client())}
	err := p.processJob(context.Background(), store.AccrualJob{ID: 1, Number: "1"})
	if err != nil {
		t.Fatal(err)
	}
}

func TestProcessJob_rateLimitedShortRetry(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "1")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()
	st := &stubStore{}
	p := &AccrualPoller{Store: st, Client: accrual.NewClient(srv.URL, srv.Client())}
	err := p.processJob(context.Background(), store.AccrualJob{ID: 1, Number: "1"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestProcessJob_clientError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	st := &stubStore{}
	p := &AccrualPoller{Store: st, Client: accrual.NewClient(srv.URL, srv.Client())}
	err := p.processJob(context.Background(), store.AccrualJob{ID: 1, Number: "1"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestProcessJob_syncOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"order":"12345678903","status":"PROCESSED","accrual":100}`))
	}))
	defer srv.Close()
	st := &stubStore{}
	p := &AccrualPoller{Store: st, Client: accrual.NewClient(srv.URL, srv.Client())}
	err := p.processJob(context.Background(), store.AccrualJob{ID: 7, Number: "12345678903"})
	if err != nil {
		t.Fatal(err)
	}
	if st.syncCalls != 1 {
		t.Fatalf("sync calls %d", st.syncCalls)
	}
}

func TestTick_processJobFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"order":"12345678903","status":"PROCESSED","accrual":100}`))
	}))
	defer srv.Close()
	st := &stubStore{
		jobs:    []store.AccrualJob{{ID: 1, Number: "12345678903"}},
		syncErr: errors.New("sync failed"),
	}
	p := &AccrualPoller{Store: st, Client: accrual.NewClient(srv.URL, srv.Client()), Batch: 10}
	p.tick(context.Background())
}

func TestTick_processesJobs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	st := &stubStore{jobs: []store.AccrualJob{{ID: 1, Number: "1"}}}
	p := &AccrualPoller{Store: st, Client: accrual.NewClient(srv.URL, srv.Client()), Batch: 10}
	p.tick(context.Background())
}
