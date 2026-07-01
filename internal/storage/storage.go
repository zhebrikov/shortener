// Package storage определяет модель ссылки и реализации хранилища (файл, память, PostgreSQL).
package storage

import (
	"encoding/json"
	"errors"
	"log"
	"os"
	"strings"
	"sync"
)

// ErrDuplicateURL возвращается при попытке сохранить URL, который уже есть в хранилище (уникальное нарушение).
var ErrDuplicateURL = errors.New("duplicate url")

// ErrLinkNotFound возвращается, если ссылка с указанным shortCode не найдена.
var ErrLinkNotFound = errors.New("link not found")

var marshalStorageLinks = json.MarshalIndent

var createStorageFile = func(filename string) error {
	return os.WriteFile(filename, []byte("[]"), 0644)
}

// LinkStore — интерфейс хранилища ссылок (БД, файл или память).
type LinkStore interface {
	// ReadStorage возвращает все сохранённые ссылки.
	ReadStorage() ([]Link, error)
	// WriteStorage сохраняет одну ссылку; при дубликате originalURL возвращает ErrDuplicateURL.
	WriteStorage(link Link) error
	// WriteStorageBatch сохраняет несколько ссылок атомарно (одна транзакция/один запрос).
	WriteStorageBatch(links []Link) error
	// GetShortURLByOriginalURL возвращает уже имеющийся сокращённый URL по оригиналу или ошибку.
	GetShortURLByOriginalURL(originalURL string) (string, error)
	// GetLinksByUserID возвращает все ссылки, созданные пользователем с данным идентификатором.
	GetLinksByUserID(userID string) ([]Link, error)
	// SoftDeleteURLsByUser помечает ссылки как удалённые (только принадлежащие userID).
	SoftDeleteURLsByUser(userID string, shortCodes []string) error
	// GetLinkByShortCode возвращает запись по коду из пути (суффикс short_url).
	GetLinkByShortCode(shortCode string) (Link, error)
	// CountURLs возвращает общее количество сокращённых URL в хранилище.
	CountURLs() (int, error)
	// CountUsers возвращает количество уникальных пользователей в хранилище.
	CountUsers() (int, error)
}

// Storage хранит ссылки в JSON-файле на диске.
type Storage struct {
	mu       sync.Mutex
	filename string
}

// Проверка, что *Storage реализует LinkStore.
var _ LinkStore = (*Storage)(nil)

// Link — запись о сокращённой ссылке (оригинал, short URL, владелец, флаг удаления).
type Link struct {
	UUID        int    `json:"uuid"`
	ShortURL    string `json:"short_url"`
	OriginalURL string `json:"original_url"`
	UserID      string `json:"user_id,omitempty"`
	IsDeleted   bool   `json:"is_deleted,omitempty"`
}

// ShortCodeFromURL извлекает код из полного short URL (последний сегмент пути).
func ShortCodeFromURL(shortURL string) string {
	if i := strings.LastIndex(shortURL, "/"); i >= 0 {
		return shortURL[i+1:]
	}
	return shortURL
}

// LinkMatchesShortCode проверяет, что shortCode совпадает с сохранённым значением short_url (полный URL или суффикс /code).
func LinkMatchesShortCode(storedShortURL, shortCode string) bool {
	if shortCode == "" {
		return false
	}
	return storedShortURL == shortCode || strings.HasSuffix(storedShortURL, "/"+shortCode)
}

// NewStorage создаёт файловое хранилище по пути filename.
func NewStorage(filename string) *Storage {
	return &Storage{filename: filename}
}

// ReadStorage читает все ссылки из файла.
func (s *Storage) ReadStorage() ([]Link, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.readStorageLocked()
}

// WriteStorage дописывает ссылку в файл; дубликат originalURL даёт ErrDuplicateURL.
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
			if err := createStorageFile(s.filename); err != nil {
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
	data, err := marshalStorageLinks(links, "", "  ")
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

// GetShortURLByOriginalURL возвращает short URL по оригинальному адресу.
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

// GetLinksByUserID возвращает неудалённые ссылки пользователя.
func (s *Storage) GetLinksByUserID(userID string) ([]Link, error) {
	if userID == "" {
		return nil, nil
	}
	links, err := s.ReadStorage()
	if err != nil {
		return nil, err
	}
	var out []Link
	for _, l := range links {
		if l.UserID == userID && !l.IsDeleted {
			out = append(out, l)
		}
	}
	return out, nil
}

// SoftDeleteURLsByUser помечает ссылки пользователя как удалённые.
func (s *Storage) SoftDeleteURLsByUser(userID string, shortCodes []string) error {
	if userID == "" || len(shortCodes) == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	links, err := s.readStorageLocked()
	if err != nil {
		return err
	}
	changed := false
	for i := range links {
		if links[i].UserID != userID || links[i].IsDeleted {
			continue
		}
		for _, code := range shortCodes {
			if LinkMatchesShortCode(links[i].ShortURL, code) {
				links[i].IsDeleted = true
				changed = true
				break
			}
		}
	}
	if !changed {
		return nil
	}
	return s.writeAllStorageLocked(links)
}

// GetLinkByShortCode возвращает запись по коду из пути или ErrLinkNotFound.
func (s *Storage) GetLinkByShortCode(shortCode string) (Link, error) {
	if shortCode == "" {
		return Link{}, ErrLinkNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	links, err := s.readStorageLocked()
	if err != nil {
		return Link{}, err
	}
	for _, l := range links {
		if LinkMatchesShortCode(l.ShortURL, shortCode) {
			return l, nil
		}
	}
	return Link{}, ErrLinkNotFound
}

// CountURLs возвращает общее количество ссылок в файле.
func (s *Storage) CountURLs() (int, error) {
	links, err := s.ReadStorage()
	if err != nil {
		return 0, err
	}
	return len(links), nil
}

// CountUsers возвращает количество уникальных пользователей.
func (s *Storage) CountUsers() (int, error) {
	links, err := s.ReadStorage()
	if err != nil {
		return 0, err
	}
	users := make(map[string]struct{})
	for _, l := range links {
		if l.UserID != "" {
			users[l.UserID] = struct{}{}
		}
	}
	return len(users), nil
}
