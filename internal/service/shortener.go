package service

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
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
			return "http://" + s.baseURL + "/" + shortCode
		}
	}
}

func (s *Shortener) GetLink(shortCode string) *string {
	url, ok := s.repoLink[shortCode]
	if !ok {
		return nil
	}
	return &url
}
