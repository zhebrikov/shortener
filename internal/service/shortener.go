package service

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/zhebrikov/shortener/internal/storage"
)

type Shortener struct {
	repoLink map[string]string
	baseURL  string
}

func NewShortener(baseURL string) *Shortener {
	return &Shortener{
		repoLink: make(map[string]string),
		baseURL:  baseURL,
	}
}

func (s *Shortener) shortURL(shortCode string) string {
	if strings.HasPrefix(s.baseURL, "http://") || strings.HasPrefix(s.baseURL, "https://") {
		return strings.TrimSuffix(s.baseURL, "/") + "/" + shortCode
	}
	return "http://" + strings.TrimSuffix(s.baseURL, "/") + "/" + shortCode
}

func (s *Shortener) CreateLink(originalURL string) string {
	for i := 0; ; i++ {
		input := originalURL
		if i > 0 {
			input = fmt.Sprintf("%s_%d", originalURL, i)
		}

		hash := sha1.Sum([]byte(input))
		shortCode := hex.EncodeToString(hash[:])[:8]

		if _, exists := s.repoLink[shortCode]; !exists {
			s.repoLink[shortCode] = originalURL
			return s.shortURL(shortCode)
		}
	}
}

func (s *Shortener) GetLink(shortCode string, store *storage.Storage) *string {
	links, err := store.ReadStorage()
	if err != nil {
		return nil
	}
	for _, link := range links {
		// ShortURL в storage может быть полный URL или только shortCode
		if link.ShortURL == shortCode || strings.HasSuffix(link.ShortURL, "/"+shortCode) {
			return &link.OriginalURL
		}
	}
	return nil
}
