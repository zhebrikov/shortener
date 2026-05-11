// Package worker polls the accrual system and updates local order rows until they reach a terminal state.
package worker

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/zhebrikov/shortener/internal/gophermart/accrual"
	"github.com/zhebrikov/shortener/internal/gophermart/store"
)

// AccrualPoller periodically synchronizes pending orders with the accrual HTTP API.
type AccrualPoller struct {
	Store  store.AccrualStore
	Client *accrual.Client
	// Interval between polling rounds when idle.
	Interval time.Duration
	// Batch is the maximum number of orders to inspect per tick.
	Batch int
	// Sleep blocks for the given duration after the accrual API responds with
	// 429 Too Many Requests. When nil [time.Sleep] is used; tests override it
	// to avoid waiting on real timers.
	Sleep func(time.Duration)
}

// Run blocks until ctx is cancelled, logging non-fatal errors to the standard logger.
func (p *AccrualPoller) Run(ctx context.Context) {
	if p.Interval <= 0 {
		p.Interval = 2 * time.Second
	}
	if p.Batch <= 0 {
		p.Batch = 20
	}
	t := time.NewTicker(p.Interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			p.tick(ctx)
		}
	}
}

func (p *AccrualPoller) tick(ctx context.Context) {
	jobs, err := p.Store.PendingAccrualJobs(ctx, p.Batch)
	if err != nil {
		log.Printf("accrual worker: pending jobs: %v", err)
		return
	}
	for _, j := range jobs {
		if err := p.processJob(ctx, j); err != nil {
			log.Printf("accrual worker: order %s: %v", j.Number, err)
		}
	}
}

func (p *AccrualPoller) processJob(ctx context.Context, j store.AccrualJob) error {
	info, err := p.Client.GetOrder(j.Number)
	if errors.Is(err, accrual.ErrNotRegistered) {
		return nil
	}
	var tooMany *accrual.ErrTooManyRequests
	if errors.As(err, &tooMany) {
		if tooMany.RetryAfter > 0 {
			p.sleep(tooMany.RetryAfter)
		} else {
			p.sleep(time.Minute)
		}
		return err
	}
	if err != nil {
		return err
	}

	localStatus, acc := mapAccrualResponse(info)
	return p.Store.SyncAccrualStatus(ctx, j.ID, localStatus, acc)
}

func mapAccrualResponse(info *accrual.OrderInfo) (localStatus string, accrualPtr *float64) {
	switch info.Status {
	case accrual.StatusRegistered, accrual.StatusProcessing:
		return "PROCESSING", nil
	case accrual.StatusInvalid:
		return "INVALID", nil
	case accrual.StatusProcessed:
		return "PROCESSED", info.Accrual
	default:
		return "PROCESSING", nil
	}
}

func (p *AccrualPoller) sleep(d time.Duration) {
	if p.Sleep != nil {
		p.Sleep(d)
		return
	}
	time.Sleep(d)
}
