package service

import (
	"crypto/sha1"
	"encoding/hex"
)

type Shortener struct {
	repoLink map[string]string
}

func NewShortener() *Shortener {
	return &Shortener{
		repoLink: make(map[string]string),
	}
}

func (s *Shortener) CreateLink(originalURL string) string {
	hash := sha1.Sum([]byte(originalURL))
	shortCode := hex.EncodeToString(hash[:])[:8]

	s.repoLink[shortCode] = originalURL

	return shortCode
}

func (s *Shortener) GetLink(shortCode string) *string {
	url, ok := s.repoLink[shortCode]
	if !ok {
		return nil
	}
	return &url
}
