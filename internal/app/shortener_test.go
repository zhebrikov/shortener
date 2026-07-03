package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/zhebrikov/shortener/internal/audit"
	"github.com/zhebrikov/shortener/internal/auth"
	"github.com/zhebrikov/shortener/internal/service"
	"github.com/zhebrikov/shortener/internal/storage"
)

type stubStore struct {
	links []storage.Link
}

func (s *stubStore) ReadStorage() ([]storage.Link, error) { return s.links, nil }
func (s *stubStore) WriteStorage(l storage.Link) error {
	for _, existing := range s.links {
		if existing.OriginalURL == l.OriginalURL {
			return storage.ErrDuplicateURL
		}
	}
	s.links = append(s.links, l)
	return nil
}
func (s *stubStore) WriteAllStorage(links []storage.Link) error { s.links = links; return nil }
func (s *stubStore) WriteStorageBatch(links []storage.Link) error {
	for _, l := range links {
		if err := s.WriteStorage(l); err != nil {
			return err
		}
	}
	return nil
}
func (s *stubStore) GetShortURLByOriginalURL(original string) (string, error) {
	for _, l := range s.links {
		if l.OriginalURL == original {
			return l.ShortURL, nil
		}
	}
	return "", storage.ErrLinkNotFound
}
func (s *stubStore) GetLinksByUserID(userID string) ([]storage.Link, error) {
	if userID == "store-error" {
		return nil, errors.New("store failure")
	}
	var out []storage.Link
	for _, l := range s.links {
		if l.UserID == userID {
			out = append(out, l)
		}
	}
	return out, nil
}
func (s *stubStore) SoftDeleteURLsByUser(string, []string) error { return nil }
func (s *stubStore) GetLinkByShortCode(shortCode string) (storage.Link, error) {
	for _, l := range s.links {
		if storage.LinkMatchesShortCode(l.ShortURL, shortCode) {
			return l, nil
		}
	}
	return storage.Link{}, storage.ErrLinkNotFound
}
func (s *stubStore) CountURLs() (int, error)  { return len(s.links), nil }
func (s *stubStore) CountUsers() (int, error) { return 0, nil }
func (s *stubStore) Stats() (int, int, error) { return len(s.links), 0, nil }

func TestShortenerApp_ShortenAndExpand(t *testing.T) {
	store := &stubStore{}
	shortener := service.NewShortener("http://localhost:8080")
	a := NewShortenerApp(shortener, store, nil)

	result, err := a.ShortenURL(context.Background(), "https://example.com/app")
	if err != nil {
		t.Fatal(err)
	}
	if result.IsConflict || result.ShortURL == "" {
		t.Fatalf("result = %+v", result)
	}

	shortCode := storage.ShortCodeFromURL(result.ShortURL)
	original, err := a.ExpandURL(context.Background(), shortCode)
	if err != nil || original != "https://example.com/app" {
		t.Fatalf("ExpandURL() = %q, %v", original, err)
	}
}

func TestShortenerApp_ShortenURL_conflict(t *testing.T) {
	store := &stubStore{}
	shortener := service.NewShortener("http://localhost:8080")
	a := NewShortenerApp(shortener, store, nil)

	first, err := a.ShortenURL(context.Background(), "https://example.com/dup")
	if err != nil {
		t.Fatal(err)
	}
	second, err := a.ShortenURL(context.Background(), "https://example.com/dup")
	if err != nil {
		t.Fatal(err)
	}
	if !second.IsConflict || second.ShortURL != first.ShortURL {
		t.Fatalf("second = %+v, first = %+v", second, first)
	}
}

func TestShortenerApp_ListUserURLs_authErrors(t *testing.T) {
	a := NewShortenerApp(service.NewShortener("http://localhost:8080"), &stubStore{}, nil)

	if _, err := a.ListUserURLs(auth.WithEmptyAuthCookie(context.Background())); !errors.Is(err, ErrEmptyAuth) {
		t.Fatalf("err = %v", err)
	}
	if _, err := a.ListUserURLs(context.Background()); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("err = %v", err)
	}
}

func TestShortenerApp_ListUserURLs_success(t *testing.T) {
	store := &stubStore{}
	shortener := service.NewShortener("http://localhost:8080")
	a := NewShortenerApp(shortener, store, nil)

	ctx := auth.WithUserID(context.Background(), "user-1")
	result, err := a.ShortenURL(ctx, "https://example.com/mine")
	if err != nil {
		t.Fatal(err)
	}

	links, err := a.ListUserURLs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(links) != 1 || links[0].ShortURL != result.ShortURL {
		t.Fatalf("links = %+v", links)
	}
}

func TestShortenerApp_ListUserURLs_storeError(t *testing.T) {
	a := NewShortenerApp(service.NewShortener("http://localhost:8080"), &stubStore{}, nil)
	ctx := auth.WithUserID(context.Background(), "store-error")
	if _, err := a.ListUserURLs(ctx); err == nil {
		t.Fatal("expected store error")
	}
}

func TestShortenerApp_ShortenURL_createLinkError(t *testing.T) {
	a := NewShortenerApp(service.NewShortener("http://%zz"), &stubStore{}, nil)
	if _, err := a.ShortenURL(context.Background(), "https://example.com/bad"); err == nil {
		t.Fatal("expected create link error")
	}
}

func TestShortenerApp_ShortenURL_writeError(t *testing.T) {
	a := NewShortenerApp(service.NewShortener("http://localhost:8080"), writeErrorStore{}, nil)
	if _, err := a.ShortenURL(context.Background(), "https://example.com/write-error"); err == nil {
		t.Fatal("expected write error")
	}
}

func TestShortenerApp_ShortenURL_duplicateLookupError(t *testing.T) {
	a := NewShortenerApp(service.NewShortener("http://localhost:8080"), dupLookupErrorStore{}, nil)
	if _, err := a.ShortenURL(context.Background(), "https://example.com/dup-lookup"); err == nil {
		t.Fatal("expected duplicate lookup error")
	}
}

func TestShortenerApp_ExpandURL_notFound(t *testing.T) {
	a := NewShortenerApp(service.NewShortener("http://localhost:8080"), &stubStore{}, nil)
	if _, err := a.ExpandURL(context.Background(), "missing"); err == nil {
		t.Fatal("expected not found error")
	}
}

type writeErrorStore struct{}

func (writeErrorStore) WriteStorage(storage.Link) error { return errors.New("write failed") }
func (writeErrorStore) ReadStorage() ([]storage.Link, error) { return nil, nil }
func (writeErrorStore) WriteAllStorage([]storage.Link) error { return nil }
func (writeErrorStore) WriteStorageBatch([]storage.Link) error { return nil }
func (writeErrorStore) GetShortURLByOriginalURL(string) (string, error) { return "", storage.ErrLinkNotFound }
func (writeErrorStore) GetLinksByUserID(string) ([]storage.Link, error) { return nil, nil }
func (writeErrorStore) SoftDeleteURLsByUser(string, []string) error { return nil }
func (writeErrorStore) GetLinkByShortCode(string) (storage.Link, error) {
	return storage.Link{}, storage.ErrLinkNotFound
}
func (writeErrorStore) CountURLs() (int, error)  { return 0, nil }
func (writeErrorStore) CountUsers() (int, error) { return 0, nil }
func (writeErrorStore) Stats() (int, int, error) { return 0, 0, nil }

type dupLookupErrorStore struct{}

func (dupLookupErrorStore) WriteStorage(storage.Link) error { return storage.ErrDuplicateURL }
func (dupLookupErrorStore) GetShortURLByOriginalURL(string) (string, error) {
	return "", errors.New("lookup failed")
}
func (dupLookupErrorStore) ReadStorage() ([]storage.Link, error) { return nil, nil }
func (dupLookupErrorStore) WriteAllStorage([]storage.Link) error { return nil }
func (dupLookupErrorStore) WriteStorageBatch([]storage.Link) error { return nil }
func (dupLookupErrorStore) GetLinksByUserID(string) ([]storage.Link, error) { return nil, nil }
func (dupLookupErrorStore) SoftDeleteURLsByUser(string, []string) error { return nil }
func (dupLookupErrorStore) GetLinkByShortCode(string) (storage.Link, error) {
	return storage.Link{}, storage.ErrLinkNotFound
}
func (dupLookupErrorStore) CountURLs() (int, error)  { return 0, nil }
func (dupLookupErrorStore) CountUsers() (int, error) { return 0, nil }
func (dupLookupErrorStore) Stats() (int, int, error) { return 0, 0, nil }

func TestShortenerApp_auditPublished(t *testing.T) {
	store := &stubStore{}
	shortener := service.NewShortener("http://localhost:8080")
	ch := make(chan audit.Event, 1)
	pub := audit.NewPublisher(auditChanObserver{ch: ch})
	a := NewShortenerApp(shortener, store, pub)

	ctx := auth.WithUserID(context.Background(), "audit-user")
	if _, err := a.ShortenURL(ctx, "https://example.com/audit"); err != nil {
		t.Fatal(err)
	}
	select {
	case ev := <-ch:
		if ev.Action != audit.ActionShorten || ev.UserID != "audit-user" {
			t.Fatalf("event = %+v", ev)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected audit event")
	}
}

type auditChanObserver struct {
	ch chan audit.Event
}

func (o auditChanObserver) OnAudit(_ context.Context, ev audit.Event) error {
	o.ch <- ev
	return nil
}
