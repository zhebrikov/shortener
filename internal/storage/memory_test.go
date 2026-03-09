package storage

import (
	"testing"
)

func TestNewMemoryStorage(t *testing.T) {
	m := NewMemoryStorage()
	if m == nil {
		t.Fatal("NewMemoryStorage returned nil")
	}
	links, err := m.ReadStorage()
	if err != nil {
		t.Fatalf("ReadStorage() err = %v", err)
	}
	if links == nil {
		t.Fatal("ReadStorage() returned nil slice")
	}
	if len(links) != 0 {
		t.Errorf("len(links) = %d, want 0", len(links))
	}
}

func TestMemoryStorage_ReadStorage_Empty(t *testing.T) {
	m := NewMemoryStorage()
	links, err := m.ReadStorage()
	if err != nil {
		t.Fatalf("ReadStorage() err = %v", err)
	}
	if len(links) != 0 {
		t.Errorf("len(links) = %d, want 0", len(links))
	}
}

func TestMemoryStorage_WriteStorage(t *testing.T) {
	m := NewMemoryStorage()
	link := Link{UUID: 1, ShortURL: "abc", OriginalURL: "https://example.com"}

	if err := m.WriteStorage(link); err != nil {
		t.Fatalf("WriteStorage() err = %v", err)
	}

	links, err := m.ReadStorage()
	if err != nil {
		t.Fatalf("ReadStorage() err = %v", err)
	}
	if len(links) != 1 {
		t.Fatalf("len(links) = %d, want 1", len(links))
	}
	if links[0].UUID != link.UUID || links[0].ShortURL != link.ShortURL || links[0].OriginalURL != link.OriginalURL {
		t.Errorf("links[0] = %+v, want %+v", links[0], link)
	}
}

func TestMemoryStorage_WriteStorage_Appends(t *testing.T) {
	m := NewMemoryStorage()
	_ = m.WriteStorage(Link{UUID: 1, ShortURL: "a", OriginalURL: "https://a.com"})
	_ = m.WriteStorage(Link{UUID: 2, ShortURL: "b", OriginalURL: "https://b.com"})

	links, err := m.ReadStorage()
	if err != nil {
		t.Fatalf("ReadStorage() err = %v", err)
	}
	if len(links) != 2 {
		t.Fatalf("len(links) = %d, want 2", len(links))
	}
	if links[0].ShortURL != "a" || links[1].ShortURL != "b" {
		t.Errorf("links = %+v", links)
	}
}

func TestMemoryStorage_ReadStorage_ReturnsCopy(t *testing.T) {
	m := NewMemoryStorage()
	_ = m.WriteStorage(Link{UUID: 1, ShortURL: "x", OriginalURL: "https://x.com"})

	links1, _ := m.ReadStorage()
	links2, _ := m.ReadStorage()
	if len(links1) == 0 || len(links2) == 0 {
		t.Fatal("expected non-empty slices")
	}
	// Изменение результата не должно влиять на хранилище
	links1[0].ShortURL = "mutated"
	links2, _ = m.ReadStorage()
	if links2[0].ShortURL != "x" {
		t.Errorf("ReadStorage() should return copy; got short_url = %q", links2[0].ShortURL)
	}
}
