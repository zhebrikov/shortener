package service

import (
	"crypto/sha1"
	"errors"
	"strings"
	"testing"

	"github.com/zhebrikov/shortener/internal/storage"
)

func TestNewShortener(t *testing.T) {
	s := NewShortener("example.com")
	if s == nil || s.baseURL != "example.com" {
		t.Fatalf("NewShortener: %+v", s)
	}
}

func TestShortener_shortURL_withScheme(t *testing.T) {
	s := NewShortener("https://go.dev")
	got, err := s.shortURL("abc")
	if err != nil {
		t.Fatal(err)
	}
	if want := "https://go.dev/abc"; got != want {
		t.Errorf("shortURL = %q, want %q", got, want)
	}
}

func TestShortener_shortURL_withoutScheme(t *testing.T) {
	s := NewShortener("localhost:8080")
	got, err := s.shortURL("x1")
	if err != nil {
		t.Fatal(err)
	}
	if want := "http://localhost:8080/x1"; got != want {
		t.Errorf("shortURL = %q, want %q", got, want)
	}
}

func TestShortCodeFromHash(t *testing.T) {
	h := [20]byte{0xde, 0xad, 0xbe, 0xef}
	code := shortCodeFromHash(h)
	if len(code) != 8 {
		t.Errorf("len = %d, want 8", len(code))
	}
}

func TestShortener_CreateLink(t *testing.T) {
	s := NewShortener("http://127.0.0.1:9")
	u, err := s.CreateLink("https://example.com/path")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(u, "http://127.0.0.1:9/") {
		t.Errorf("CreateLink = %q", u)
	}
}

func TestShortener_CreateLink_collisionRetry(t *testing.T) {
	s := NewShortener("http://127.0.0.1:9")
	orig := "https://collision.example/a"
	hash := sha1.Sum([]byte(orig))
	code := shortCodeFromHash(hash)
	s.mu.Lock()
	s.repoLink[code] = "https://other.com"
	s.mu.Unlock()

	u, err := s.CreateLink(orig)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(u, "http://127.0.0.1:9/") {
		t.Fatalf("unexpected short URL: %q", u)
	}
}

func TestShortener_shortURL_invalidBase(t *testing.T) {
	s := NewShortener("http://%zz")
	_, err := s.shortURL("code")
	if err == nil {
		t.Fatal("expected error from url.JoinPath")
	}
}

func TestShortener_shortURL_plainBaseJoinPathError(t *testing.T) {
	s := NewShortener("%")
	_, err := s.shortURL("c")
	if err == nil {
		t.Fatal("expected error from url.JoinPath on plain base branch")
	}
}

func TestShortener_CreateLink_invalidBaseURL(t *testing.T) {
	s := NewShortener("http://%zz")
	_, err := s.CreateLink("https://example.com/x")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestShortener_GetLink_success(t *testing.T) {
	mem := storage.NewMemoryStorage()
	_ = mem.WriteStorage(storage.Link{
		UUID:        1,
		ShortURL:    "http://base/c1",
		OriginalURL: "https://orig",
	})
	s := NewShortener("http://base")
	got, err := s.GetLink("c1", mem)
	if err != nil || got != "https://orig" {
		t.Fatalf("GetLink = %q, %v", got, err)
	}
}

func TestShortener_GetLink_notFound(t *testing.T) {
	s := NewShortener("http://base")
	_, err := s.GetLink("nope", storage.NewMemoryStorage())
	if !errors.Is(err, ErrLinkNotFound) {
		t.Fatalf("err = %v", err)
	}
}

func TestShortener_GetLink_deleted(t *testing.T) {
	mem := storage.NewMemoryStorage()
	_ = mem.WriteStorage(storage.Link{
		UUID:        1,
		ShortURL:    "http://base/d1",
		OriginalURL: "https://x",
		IsDeleted:   true,
	})
	s := NewShortener("http://base")
	_, err := s.GetLink("d1", mem)
	if !errors.Is(err, ErrLinkDeleted) {
		t.Fatalf("err = %v", err)
	}
}

func TestShortener_GetLink_storeError(t *testing.T) {
	s := NewShortener("http://base")
	want := errors.New("boom")
	_, err := s.GetLink("any", errStore{err: want})
	if !errors.Is(err, want) {
		t.Fatalf("err = %v", err)
	}
}

type errStore struct {
	err error
}

func (e errStore) ReadStorage() ([]storage.Link, error)            { return nil, e.err }
func (e errStore) WriteStorage(storage.Link) error                 { return e.err }
func (e errStore) WriteStorageBatch([]storage.Link) error          { return e.err }
func (e errStore) GetShortURLByOriginalURL(string) (string, error) { return "", e.err }
func (e errStore) GetLinksByUserID(string) ([]storage.Link, error) { return nil, e.err }
func (e errStore) SoftDeleteURLsByUser(string, []string) error     { return e.err }
func (e errStore) GetLinkByShortCode(string) (storage.Link, error) { return storage.Link{}, e.err }
