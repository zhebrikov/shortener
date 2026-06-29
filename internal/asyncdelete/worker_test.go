package asyncdelete

import (
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/zhebrikov/shortener/internal/storage"
)

func TestSubmit_noop(t *testing.T) {
	store := storage.NewMemoryStorage()
	w := NewWorker(store)
	w.Submit("", []string{"a"})
	w.Submit("u1", nil)
	w.Submit("u1", []string{"", "  "})
	w.Shutdown()
}

func TestWorker_softDelete(t *testing.T) {
	store := storage.NewMemoryStorage()
	_ = store.WriteStorage(storage.Link{
		UUID: 1, ShortURL: "http://localhost/c1", OriginalURL: "https://a.com", UserID: "u1",
	})
	w := NewWorker(store)
	w.Submit("u1", []string{"c1", "c1"})

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		link, err := store.GetLinkByShortCode("c1")
		if err == nil && link.IsDeleted {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("link was not soft-deleted in time")
}

func TestDedupeStrings(t *testing.T) {
	got := dedupeStrings([]string{"a", "a", "", "b", "b"})
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("dedupeStrings = %v", got)
	}
}

func TestBatchLen(t *testing.T) {
	if batchLen(map[string][]string{"u1": {"a", "b"}, "u2": {"c"}}) != 3 {
		t.Fatal("batchLen mismatch")
	}
}

type errDeleteStore struct {
	*storage.MemoryStorage
}

func (e *errDeleteStore) SoftDeleteURLsByUser(string, []string) error {
	return errors.New("delete failed")
}

func TestWorker_largeBatchFlush(t *testing.T) {
	store := storage.NewMemoryStorage()
	for i := 0; i < 300; i++ {
		code := "c" + strconv.Itoa(i)
		_ = store.WriteStorage(storage.Link{
			UUID: i + 1, ShortURL: "http://localhost/" + code,
			OriginalURL: "https://example.com/" + code, UserID: "u-batch",
		})
	}
	w := NewWorker(store)
	for i := 0; i < 300; i++ {
		w.Submit("u-batch", []string{"c" + strconv.Itoa(i)})
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		links, err := store.GetLinksByUserID("u-batch")
		if err == nil && len(links) == 0 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("expected all user links to be soft-deleted")
}

func TestWorker_shutdownFlushesPending(t *testing.T) {
	store := storage.NewMemoryStorage()
	_ = store.WriteStorage(storage.Link{
		UUID: 1, ShortURL: "http://localhost/c1", OriginalURL: "https://a.com", UserID: "u1",
	})
	w := NewWorker(store)
	w.Submit("u1", []string{"c1"})
	w.Shutdown()

	link, err := store.GetLinkByShortCode("c1")
	if err != nil {
		t.Fatal(err)
	}
	if !link.IsDeleted {
		t.Fatal("expected link to be soft-deleted after Shutdown")
	}
}

func TestWorker_flushError(t *testing.T) {
	store := &errDeleteStore{MemoryStorage: storage.NewMemoryStorage()}
	w := NewWorker(store)
	w.Submit("u1", []string{"c1"})
	time.Sleep(150 * time.Millisecond)
}
