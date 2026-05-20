package service

import (
	"fmt"
	"testing"

	"github.com/zhebrikov/shortener/internal/storage"
)

func BenchmarkShortener_CreateLink(b *testing.B) {
	s := NewShortener("http://localhost:8080")
	url := "https://example.com/page"
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := s.CreateLink(url); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkShortener_GetLink(b *testing.B) {
	s := NewShortener("http://localhost:8080")
	store := storage.NewMemoryStorage()
	const n = 5000
	var target string
	for i := 0; i < n; i++ {
		original := fmt.Sprintf("https://example.com/%d", i)
		shortURL, err := s.CreateLink(original)
		if err != nil {
			b.Fatal(err)
		}
		if err := store.WriteStorage(storage.Link{
			UUID:        i + 1,
			ShortURL:    shortURL,
			OriginalURL: original,
		}); err != nil {
			b.Fatal(err)
		}
		if i == n/2 {
			target = storage.ShortCodeFromURL(shortURL)
		}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := s.GetLink(target, store); err != nil {
			b.Fatal(err)
		}
	}
}
