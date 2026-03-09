package handler

import (
	"os"
	"path/filepath"
	"testing"

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
