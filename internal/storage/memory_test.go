package storage

import (
	"errors"
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

func TestMemoryStorage_GetLinkByShortCode(t *testing.T) {
	m := NewMemoryStorage()
	link := Link{UUID: 1, ShortURL: "http://localhost/abc12345", OriginalURL: "https://example.com"}
	if err := m.WriteStorage(link); err != nil {
		t.Fatal(err)
	}
	got, err := m.GetLinkByShortCode("abc12345")
	if err != nil {
		t.Fatalf("GetLinkByShortCode() err = %v", err)
	}
	if got.OriginalURL != link.OriginalURL {
		t.Errorf("GetLinkByShortCode() = %+v, want %+v", got, link)
	}
	if _, err := m.GetLinkByShortCode("missing"); err != ErrLinkNotFound {
		t.Errorf("GetLinkByShortCode(missing) err = %v, want ErrLinkNotFound", err)
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

func TestMemoryStorage_WriteStorageBatch(t *testing.T) {
	m := NewMemoryStorage()
	links := []Link{
		{UUID: 1, ShortURL: "http://localhost/a", OriginalURL: "https://a.com"},
		{UUID: 2, ShortURL: "http://localhost/b", OriginalURL: "https://b.com"},
	}
	if err := m.WriteStorageBatch(links); err != nil {
		t.Fatal(err)
	}
	if err := m.WriteStorageBatch(nil); err != nil {
		t.Fatal(err)
	}
	got, _ := m.ReadStorage()
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
}

func TestMemoryStorage_GetShortURLByOriginalURL(t *testing.T) {
	m := NewMemoryStorage()
	_ = m.WriteStorage(Link{UUID: 1, ShortURL: "http://localhost/x", OriginalURL: "https://x.com"})
	got, err := m.GetShortURLByOriginalURL("https://x.com")
	if err != nil || got != "http://localhost/x" {
		t.Fatalf("got %q, %v", got, err)
	}
	if _, err := m.GetShortURLByOriginalURL("https://missing.com"); err == nil {
		t.Fatal("expected error")
	}
}

func TestMemoryStorage_GetLinksByUserID(t *testing.T) {
	m := NewMemoryStorage()
	if links, err := m.GetLinksByUserID(""); err != nil || links != nil {
		t.Fatalf("empty userID: %+v, %v", links, err)
	}
	_ = m.WriteStorage(Link{UUID: 1, ShortURL: "a", OriginalURL: "https://a.com", UserID: "u1"})
	_ = m.WriteStorage(Link{UUID: 2, ShortURL: "b", OriginalURL: "https://b.com", UserID: "u2"})
	_ = m.WriteStorage(Link{UUID: 3, ShortURL: "c", OriginalURL: "https://c.com", UserID: "u1", IsDeleted: true})

	links, err := m.GetLinksByUserID("u1")
	if err != nil || len(links) != 1 || links[0].ShortURL != "a" {
		t.Fatalf("got %+v, %v", links, err)
	}
}

func TestMemoryStorage_SoftDeleteURLsByUser(t *testing.T) {
	m := NewMemoryStorage()
	_ = m.WriteStorage(Link{UUID: 1, ShortURL: "http://localhost/code1", OriginalURL: "https://a.com", UserID: "u1"})
	if err := m.SoftDeleteURLsByUser("", []string{"code1"}); err != nil {
		t.Fatal(err)
	}
	if err := m.SoftDeleteURLsByUser("u1", nil); err != nil {
		t.Fatal(err)
	}
	if err := m.SoftDeleteURLsByUser("u1", []string{"code1"}); err != nil {
		t.Fatal(err)
	}
	link, err := m.GetLinkByShortCode("code1")
	if err != nil || !link.IsDeleted {
		t.Fatalf("link = %+v, err = %v", link, err)
	}
}

func TestMemoryStorage_GetLinkByShortCode_empty(t *testing.T) {
	m := NewMemoryStorage()
	if _, err := m.GetLinkByShortCode(""); !errors.Is(err, ErrLinkNotFound) {
		t.Fatalf("err = %v", err)
	}
}

func TestMemoryStorage_GetLinkByShortCode_fullURLInPath(t *testing.T) {
	m := NewMemoryStorage()
	_ = m.WriteStorage(Link{UUID: 1, ShortURL: "http://localhost/abc", OriginalURL: "https://full.example"})
	got, err := m.GetLinkByShortCode("http://localhost/abc")
	if err != nil || got.OriginalURL != "https://full.example" {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestMemoryStorage_GetLinkByShortCode_byCodeIndex(t *testing.T) {
	m := NewMemoryStorage()
	_ = m.WriteStorage(Link{UUID: 1, ShortURL: "rawcode", OriginalURL: "https://bycode.com"})
	got, err := m.GetLinkByShortCode("rawcode")
	if err != nil || got.OriginalURL != "https://bycode.com" {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestMemoryStorage_SoftDeleteURLsByUser_wrongUser(t *testing.T) {
	m := NewMemoryStorage()
	_ = m.WriteStorage(Link{UUID: 1, ShortURL: "http://localhost/c1", OriginalURL: "https://a.com", UserID: "u1"})
	if err := m.SoftDeleteURLsByUser("u2", []string{"c1"}); err != nil {
		t.Fatal(err)
	}
	link, _ := m.GetLinkByShortCode("c1")
	if link.IsDeleted {
		t.Fatal("other user's link must not be deleted")
	}
}

func TestMemoryStorage_SoftDeleteURLsByUser_noMatchingCode(t *testing.T) {
	m := NewMemoryStorage()
	_ = m.WriteStorage(Link{UUID: 1, ShortURL: "http://localhost/c1", OriginalURL: "https://a.com", UserID: "u1"})
	if err := m.SoftDeleteURLsByUser("u1", []string{"other"}); err != nil {
		t.Fatal(err)
	}
	link, _ := m.GetLinkByShortCode("c1")
	if link.IsDeleted {
		t.Fatal("link should not be deleted")
	}
}

func TestMemoryStorage_WriteStorage_duplicate(t *testing.T) {
	m := NewMemoryStorage()
	link := Link{UUID: 1, ShortURL: "a", OriginalURL: "https://dup.com"}
	if err := m.WriteStorage(link); err != nil {
		t.Fatal(err)
	}
	if err := m.WriteStorage(link); !errors.Is(err, ErrDuplicateURL) {
		t.Fatalf("err = %v", err)
	}
}

func TestMemoryStorage_CountURLsAndUsers(t *testing.T) {
	m := NewMemoryStorage()
	urls, err := m.CountURLs()
	if err != nil || urls != 0 {
		t.Fatalf("CountURLs() = %d, %v; want 0, nil", urls, err)
	}
	users, err := m.CountUsers()
	if err != nil || users != 0 {
		t.Fatalf("CountUsers() = %d, %v; want 0, nil", users, err)
	}

	_ = m.WriteStorage(Link{UUID: 1, ShortURL: "a", OriginalURL: "https://a.com", UserID: "u1"})
	_ = m.WriteStorage(Link{UUID: 2, ShortURL: "b", OriginalURL: "https://b.com", UserID: "u2"})
	_ = m.WriteStorage(Link{UUID: 3, ShortURL: "c", OriginalURL: "https://c.com", UserID: "u1"})

	urls, err = m.CountURLs()
	if err != nil || urls != 3 {
		t.Fatalf("CountURLs() = %d, %v; want 3, nil", urls, err)
	}
	users, err = m.CountUsers()
	if err != nil || users != 2 {
		t.Fatalf("CountUsers() = %d, %v; want 2, nil", users, err)
	}
}
