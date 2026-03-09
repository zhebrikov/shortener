package storage

import (
	"encoding/json"
	"errors"
	"log"
	"os"
	"sync"
)

// ErrDuplicateURL возвращается при попытке сохранить URL, который уже есть в хранилище (уникальное нарушение).
var ErrDuplicateURL = errors.New("duplicate url")

// LinkStore — интерфейс хранилища ссылок (БД, файл или память).
type LinkStore interface {
	ReadStorage() ([]Link, error)
	WriteStorage(link Link) error
	// WriteStorageBatch сохраняет несколько ссылок атомарно (одна транзакция/один запрос).
	WriteStorageBatch(links []Link) error
	// GetShortURLByOriginalURL возвращает уже имеющийся сокращённый URL по оригиналу или ошибку.
	GetShortURLByOriginalURL(originalURL string) (string, error)
}

type Storage struct {
	mu       sync.Mutex
	filename string
}

// Проверка, что *Storage реализует LinkStore.
var _ LinkStore = (*Storage)(nil)

type Link struct {
	UUID          int    `json:"uuid"`
	ShortURL      string `json:"short_url"`
	OriginalURL   string `json:"original_url"`
	CorrelationID string `json:"correlation_id"`
}

func NewStorage(filename string) *Storage {
	return &Storage{filename: filename}
}

func (s *Storage) ReadStorage() ([]Link, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.readStorageLocked()
}

func (s *Storage) WriteStorage(link Link) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	links, err := s.readStorageLocked()
	if err != nil {
		links = []Link{}
	}
	for _, l := range links {
		if l.OriginalURL == link.OriginalURL {
			return ErrDuplicateURL
		}
	}
	links = append(links, link)
	return s.writeAllStorageLocked(links)
}

// readStorageLocked читает файл; вызывающий должен держать s.mu.
func (s *Storage) readStorageLocked() ([]Link, error) {
	data, err := os.ReadFile(s.filename)
	if err != nil {
		if os.IsNotExist(err) {
			if err := os.WriteFile(s.filename, []byte("[]"), 0644); err != nil {
				return nil, err
			}
			return []Link{}, nil
		}
		log.Println("Error reading storage", err)
		return nil, err
	}
	var storage []Link
	if err = json.Unmarshal(data, &storage); err != nil {
		return nil, err
	}
	return storage, nil
}

// writeAllStorageLocked перезаписывает файл; вызывающий должен держать s.mu.
func (s *Storage) writeAllStorageLocked(links []Link) error {
	data, err := json.MarshalIndent(links, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.filename, data, 0644)
}

// WriteAllStorage перезаписывает файл полным списком ссылок (JSON-массив).
func (s *Storage) WriteAllStorage(links []Link) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.writeAllStorageLocked(links)
}

// WriteStorageBatch атомарно дописывает ссылки в файл (одна блокировка, один запись).
func (s *Storage) WriteStorageBatch(links []Link) error {
	if len(links) == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	existing, err := s.readStorageLocked()
	if err != nil {
		existing = []Link{}
	}
	existing = append(existing, links...)
	return s.writeAllStorageLocked(existing)
}

func (s *Storage) GetShortURLByOriginalURL(originalURL string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	links, err := s.readStorageLocked()
	if err != nil {
		return "", err
	}
	for _, link := range links {
		if link.OriginalURL == originalURL {
			return link.ShortURL, nil
		}
	}
	return "", errors.New("url not found")
}
