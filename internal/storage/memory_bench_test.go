package storage

import (
	"fmt"
	"testing"
)

func BenchmarkMemoryStorage_ReadStorage(b *testing.B) {
	m := NewMemoryStorage()
	const n = 5000
	for i := 0; i < n; i++ {
		_ = m.WriteStorage(Link{
			UUID:        i + 1,
			ShortURL:    fmt.Sprintf("http://localhost/%08d", i),
			OriginalURL: fmt.Sprintf("https://example.com/%d", i),
		})
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := m.ReadStorage(); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMemoryStorage_GetLinkByShortCode(b *testing.B) {
	m := NewMemoryStorage()
	const n = 5000
	var target string
	for i := 0; i < n; i++ {
		short := fmt.Sprintf("http://localhost/%08d", i)
		_ = m.WriteStorage(Link{
			UUID:        i + 1,
			ShortURL:    short,
			OriginalURL: fmt.Sprintf("https://example.com/%d", i),
		})
		if i == n/2 {
			target = ShortCodeFromURL(short)
		}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := m.GetLinkByShortCode(target); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMemoryStorage_WriteStorage(b *testing.B) {
	m := NewMemoryStorage()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		link := Link{
			UUID:        i + 1,
			ShortURL:    fmt.Sprintf("http://localhost/%d", i),
			OriginalURL: fmt.Sprintf("https://example.com/%d", i),
		}
		if err := m.WriteStorage(link); err != nil {
			b.Fatal(err)
		}
	}
}
