package storage

import (
	"errors"
	"sync"
)

// MemoryStorage — хранилище ссылок в памяти.
type MemoryStorage struct {
	mu     sync.RWMutex
	links  []Link
	byCode map[string]int
}

// Проверка, что *MemoryStorage реализует LinkStore.
var _ LinkStore = (*MemoryStorage)(nil)

// NewMemoryStorage создаёт хранилище в памяти.
func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		links:  make([]Link, 0),
		byCode: make(map[string]int),
	}
}

// ReadStorage возвращает копию всех ссылок из памяти.
func (m *MemoryStorage) ReadStorage() ([]Link, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Link, len(m.links))
	copy(out, m.links)
	return out, nil
}

func (m *MemoryStorage) indexLink(idx int, link Link) {
	code := ShortCodeFromURL(link.ShortURL)
	if code != "" {
		m.byCode[code] = idx
	}
}

// WriteStorage добавляет ссылку; дубликат originalURL даёт ErrDuplicateURL.
func (m *MemoryStorage) WriteStorage(link Link) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, l := range m.links {
		if l.OriginalURL == link.OriginalURL {
			return ErrDuplicateURL
		}
	}
	idx := len(m.links)
	m.links = append(m.links, link)
	m.indexLink(idx, link)
	return nil
}

// WriteStorageBatch атомарно добавляет несколько ссылок под одной блокировкой.
func (m *MemoryStorage) WriteStorageBatch(links []Link) error {
	if len(links) == 0 {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	base := len(m.links)
	m.links = append(m.links, links...)
	for i, link := range links {
		m.indexLink(base+i, link)
	}
	return nil
}

// GetShortURLByOriginalURL возвращает short URL по оригинальному адресу.
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

// GetLinksByUserID возвращает неудалённые ссылки пользователя.
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

// SoftDeleteURLsByUser помечает ссылки пользователя как удалённые.
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

// GetLinkByShortCode возвращает запись по коду или ErrLinkNotFound.
func (m *MemoryStorage) GetLinkByShortCode(shortCode string) (Link, error) {
	if shortCode == "" {
		return Link{}, ErrLinkNotFound
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	if idx, ok := m.byCode[shortCode]; ok && idx < len(m.links) {
		return m.links[idx], nil
	}
	for _, l := range m.links {
		if LinkMatchesShortCode(l.ShortURL, shortCode) {
			return l, nil
		}
	}
	return Link{}, ErrLinkNotFound
}
