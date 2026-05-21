package audit

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"sync"
)

// auditAppender открывает файл для дописывания строки аудита (подменяется в тестах).
var auditAppender = func(path string) (io.WriteCloser, error) {
	return os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
}

// FileObserver дописывает JSON-событие в конец файла, каждое с новой строки.
type FileObserver struct {
	path string
	mu   sync.Mutex
}

// NewFileObserver создаёт наблюдателя, пишущего в path.
func NewFileObserver(path string) *FileObserver {
	return &FileObserver{path: path}
}

// OnAudit реализует Observer.
func (f *FileObserver) OnAudit(_ context.Context, ev Event) error {
	data, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	line := append(data, '\n')

	f.mu.Lock()
	defer f.mu.Unlock()

	file, err := auditAppender(f.path)
	if err != nil {
		return err
	}
	_, werr := file.Write(line)
	cerr := file.Close()
	if werr != nil {
		return werr
	}
	return cerr
}
