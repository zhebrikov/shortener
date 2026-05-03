package storage

import (
	"errors"
	"sync"
)

// MemoryStorage — хранилище ссылок в памяти.
type MemoryStorage struct {
	mu    sync.RWMutex
	links []Link
}

// Проверка, что *MemoryStorage реализует LinkStore.
var _ LinkStore = (*MemoryStorage)(nil)

// NewMemoryStorage создаёт хранилище в памяти.
func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{links: make([]Link, 0)}
}

func (m *MemoryStorage) ReadStorage() ([]Link, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Link, len(m.links))
	copy(out, m.links)
	return out, nil
}

func (m *MemoryStorage) WriteStorage(link Link) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, l := range m.links {
		if l.OriginalURL == link.OriginalURL {
			return ErrDuplicateURL
		}
	}
	m.links = append(m.links, link)
	return nil
}

// WriteStorageBatch атомарно добавляет несколько ссылок под одной блокировкой.
func (m *MemoryStorage) WriteStorageBatch(links []Link) error {
	if len(links) == 0 {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.links = append(m.links, links...)
	return nil
}

func (m *MemoryStorage) GetShortURLByOriginalURL(originalURL string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, link := range m.links {
		if link.OriginalURL == originalURL {
			return link.ShortURL, nil
		}
	}
	return "", errors.New("url not found")
}

func (m *MemoryStorage) GetLinksByUserID(userID string) ([]Link, error) {
	if userID == "" {
		return nil, nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []Link
	for _, l := range m.links {
		if l.UserID == userID && !l.IsDeleted {
			out = append(out, l)
		}
	}
	return out, nil
}

func (m *MemoryStorage) SoftDeleteURLsByUser(userID string, shortCodes []string) error {
	if userID == "" || len(shortCodes) == 0 {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.links {
		if m.links[i].UserID != userID || m.links[i].IsDeleted {
			continue
		}
		for _, code := range shortCodes {
			if LinkMatchesShortCode(m.links[i].ShortURL, code) {
				m.links[i].IsDeleted = true
				break
			}
		}
	}
	return nil
}
