package service

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"log"
	"net/url"
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

func (s *Shortener) shortURL(shortCode string) (string, error) {
	if strings.HasPrefix(s.baseURL, "http://") || strings.HasPrefix(s.baseURL, "https://") {
		fullURL, err := url.JoinPath(s.baseURL, shortCode)
		if err != nil {
			log.Fatal(err)
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

		if _, exists := s.repoLink[shortCode]; !exists {
			s.repoLink[shortCode] = originalURL
			shortURL, err := s.shortURL(shortCode)
			if err != nil {
				log.Fatal(err)
				return "", err
			}
			return shortURL, nil
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
