package service

import (
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"

	"github.com/zhebrikov/shortener/internal/storage"
)

type Shortener struct {
	mu       sync.Mutex
	repoLink map[string]string
	baseURL  string
}

func NewShortener(baseURL string) *Shortener {
	return &Shortener{
		repoLink: make(map[string]string),
		baseURL:  baseURL,
	}
}

func (s *Shortener) shortURL(shortCode string) (string, error) {
	if strings.HasPrefix(s.baseURL, "http://") || strings.HasPrefix(s.baseURL, "https://") {
		fullURL, err := url.JoinPath(s.baseURL, shortCode)
		if err != nil {
			return "", err
		}
		return fullURL, nil
	}

	fullURL, err := url.JoinPath("http://", s.baseURL, shortCode)
	if err != nil {
		return "", err
	}
	return fullURL, nil
}

func (s *Shortener) CreateLink(originalURL string) (string, error) {
	for i := 0; ; i++ {
		input := originalURL
		if i > 0 {
			input = fmt.Sprintf("%s_%d", originalURL, i)
		}

		hash := sha1.Sum([]byte(input))
		shortCode := hex.EncodeToString(hash[:])[:8]

		s.mu.Lock()
		_, exists := s.repoLink[shortCode]
		if !exists {
			s.repoLink[shortCode] = originalURL
		}
		s.mu.Unlock()
		if exists {
			continue
		}
		shortURL, err := s.shortURL(shortCode)
		if err != nil {
			return "", err
		}
		return shortURL, nil
	}
}

var (
	// ErrLinkNotFound означает, что ссылки с данным shortCode в хранилище нет.
	ErrLinkNotFound = errors.New("link not found")
	// ErrLinkDeleted означает, что ссылка с данным shortCode существует, но помечена как удалённая.
	ErrLinkDeleted = errors.New("link deleted")
)

// GetLink возвращает оригинальный URL.
// Ошибки:
// - ErrLinkNotFound: ссылка не найдена (нужно вернуть 404)
// - ErrLinkDeleted: ссылка помечена как удаленная (нужно вернуть 410)
// - прочие ошибки: проблемы с хранилищем/парсингом данных (нужно вернуть 500)
func (s *Shortener) GetLink(shortCode string, store storage.LinkStore) (originalURL string, err error) {
	links, err := store.ReadStorage()
	if err != nil {
		return "", err
	}
	for _, link := range links {
		if storage.LinkMatchesShortCode(link.ShortURL, shortCode) {
			if link.IsDeleted {
				return "", fmt.Errorf("%w: shortCode=%s", ErrLinkDeleted, shortCode)
			}
			return link.OriginalURL, nil
		}
	}
	return "", fmt.Errorf("%w: shortCode=%s", ErrLinkNotFound, shortCode)
}
