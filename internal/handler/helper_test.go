package handler

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/zhebrikov/shortener/internal/asyncdelete"
	"github.com/zhebrikov/shortener/internal/audit"
	"github.com/zhebrikov/shortener/internal/service"
	"github.com/zhebrikov/shortener/internal/storage"
)

func mustTempStorage(t *testing.T, content string) *storage.Storage {
	t.Helper()
	dir := t.TempDir()
	filename := filepath.Join(dir, "storage.json")
	if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return storage.NewStorage(filename)
}

func newTestHandler(shortener *service.Shortener, store storage.LinkStore, auditPub *audit.Publisher) *ShortenerHandler {
	return NewShortenerHandler(shortener, store, asyncdelete.NewWorker(store), auditPub)
}
