// Package asyncdelete выполняет отложенное мягкое удаление ссылок пользователя батчами.
package asyncdelete

import (
	"log"
	"time"

	"github.com/zhebrikov/shortener/internal/storage"
)

type item struct {
	userID string
	code   string
}

// Worker ставит задачи удаления в общую очередь.
type Worker struct {
	store       storage.LinkStore
	in          chan item
	tick        time.Duration
	maxBatchLen int
}

const (
	defaultChanBuf  = 4096
	defaultTick     = 50 * time.Millisecond
	defaultMaxBatch = 256
)

// NewWorker запускает фоновую обработку батчей SoftDeleteURLsByUser.
func NewWorker(store storage.LinkStore) *Worker {
	w := &Worker{
		store:       store,
		in:          make(chan item, defaultChanBuf),
		tick:        defaultTick,
		maxBatchLen: defaultMaxBatch,
	}
	go w.run()
	return w
}

// Submit ставит идентификаторы в очередь.
//
// Важно: горутина не создаётся на каждый вызов, чтобы не раздувать количество
// "пишущих" горутин. При заполненной очереди вызов может заблокироваться
// (backpressure).
func (w *Worker) Submit(userID string, shortCodes []string) {
	if w == nil || userID == "" || len(shortCodes) == 0 {
		return
	}
	for _, c := range shortCodes {
		if c == "" {
			continue
		}
		w.in <- item{userID: userID, code: c}
	}
}

func (w *Worker) run() {
	batch := make(map[string][]string)
	ticker := time.NewTicker(w.tick)
	defer ticker.Stop()
	for {
		select {
		case it := <-w.in:
			batch[it.userID] = append(batch[it.userID], it.code)
			if batchLen(batch) >= w.maxBatchLen {
				w.flush(batch)
				batch = make(map[string][]string, w.maxBatchLen)
			}
		case <-ticker.C:
			if len(batch) > 0 {
				w.flush(batch)
				batch = make(map[string][]string, w.maxBatchLen)
			}
		}
	}
}

func batchLen(m map[string][]string) int {
	n := 0
	for _, c := range m {
		n += len(c)
	}
	return n
}

func (w *Worker) flush(m map[string][]string) {
	for uid, codes := range m {
		if len(codes) == 0 {
			continue
		}
		uniq := dedupeStrings(codes)
		if err := w.store.SoftDeleteURLsByUser(uid, uniq); err != nil {
			log.Printf("asyncdelete: SoftDeleteURLsByUser: %v", err)
		}
	}
}

func dedupeStrings(s []string) []string {
	seen := make(map[string]struct{}, len(s))
	out := make([]string, 0, len(s))
	for _, x := range s {
		if x == "" {
			continue
		}
		if _, ok := seen[x]; ok {
			continue
		}
		seen[x] = struct{}{}
		out = append(out, x)
	}
	return out
}
