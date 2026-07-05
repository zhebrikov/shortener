package handler

import (
	"errors"

	"github.com/zhebrikov/shortener/internal/storage"
)

type stubStore struct {
	storage.LinkStore
	writeErr       error
	getShortURLErr error
}

func (s stubStore) WriteStorage(link storage.Link) error {
	if s.writeErr != nil {
		return s.writeErr
	}
	return s.LinkStore.WriteStorage(link)
}

func (s stubStore) GetShortURLByOriginalURL(originalURL string) (string, error) {
	if s.getShortURLErr != nil {
		return "", s.getShortURLErr
	}
	return s.LinkStore.GetShortURLByOriginalURL(originalURL)
}

type errReadStore struct {
	err error
}

func (e errReadStore) ReadStorage() ([]storage.Link, error)            { return nil, e.err }
func (e errReadStore) WriteStorage(storage.Link) error                 { return e.err }
func (e errReadStore) WriteStorageBatch([]storage.Link) error          { return e.err }
func (e errReadStore) GetShortURLByOriginalURL(string) (string, error) { return "", e.err }
func (e errReadStore) GetLinksByUserID(string) ([]storage.Link, error) { return nil, e.err }
func (e errReadStore) SoftDeleteURLsByUser(string, []string) error     { return e.err }
func (e errReadStore) GetLinkByShortCode(string) (storage.Link, error) {
	return storage.Link{}, e.err
}
func (e errReadStore) CountURLs() (int, error)  { return 0, e.err }
func (e errReadStore) CountUsers() (int, error) { return 0, e.err }
func (e errReadStore) Stats() (int, int, error) { return 0, 0, e.err }

type badShortenerStore struct {
	*storage.MemoryStorage
}

func (badShortenerStore) WriteStorage(storage.Link) error {
	return errors.New("write failed")
}
