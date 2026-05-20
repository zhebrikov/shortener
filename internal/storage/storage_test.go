package storage

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestNewStorage(t *testing.T) {
	s := NewStorage("/tmp/test.json")
	if s == nil {
		t.Fatal("NewStorage returned nil")
	}
	if s.filename != "/tmp/test.json" {
		t.Errorf("filename = %q, want /tmp/test.json", s.filename)
	}
}

func TestStorage_ReadStorage_FileNotExists(t *testing.T) {
	dir := t.TempDir()
	filename := filepath.Join(dir, "nonexistent.json")
	s := NewStorage(filename)

	links, err := s.ReadStorage()
	if err != nil {
		t.Fatalf("ReadStorage() err = %v", err)
	}
	if links == nil {
		t.Fatal("ReadStorage() returned nil slice")
	}
	if len(links) != 0 {
		t.Errorf("len(links) = %d, want 0", len(links))
	}
	// Должен был создаться пустой файл
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		t.Error("ReadStorage() did not create file for nonexistent path")
	}
}

func TestStorage_ReadStorage_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	filename := filepath.Join(dir, "empty.json")
	if err := os.WriteFile(filename, []byte("[]"), 0644); err != nil {
		t.Fatal(err)
	}
	s := NewStorage(filename)

	links, err := s.ReadStorage()
	if err != nil {
		t.Fatalf("ReadStorage() err = %v", err)
	}
	if len(links) != 0 {
		t.Errorf("len(links) = %d, want 0", len(links))
	}
}

func TestStorage_ReadStorage_ValidData(t *testing.T) {
	dir := t.TempDir()
	filename := filepath.Join(dir, "data.json")
	data := `[
  {
    "uuid": 1,
    "short_url": "abc",
    "original_url": "https://example.com"
  }
]`
	if err := os.WriteFile(filename, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	s := NewStorage(filename)

	links, err := s.ReadStorage()
	if err != nil {
		t.Fatalf("ReadStorage() err = %v", err)
	}
	if len(links) != 1 {
		t.Fatalf("len(links) = %d, want 1", len(links))
	}
	if links[0].UUID != 1 || links[0].ShortURL != "abc" || links[0].OriginalURL != "https://example.com" {
		t.Errorf("links[0] = %+v", links[0])
	}
}

func TestStorage_ReadStorage_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	filename := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(filename, []byte("not json"), 0644); err != nil {
		t.Fatal(err)
	}
	s := NewStorage(filename)

	_, err := s.ReadStorage()
	if err == nil {
		t.Error("ReadStorage() expected error for invalid JSON")
	}
}

func TestStorage_WriteStorage(t *testing.T) {
	dir := t.TempDir()
	filename := filepath.Join(dir, "write.json")
	s := NewStorage(filename)

	link := Link{UUID: 1, ShortURL: "x1", OriginalURL: "https://a.com"}
	if err := s.WriteStorage(link); err != nil {
		t.Fatalf("WriteStorage() err = %v", err)
	}

	links, err := s.ReadStorage()
	if err != nil {
		t.Fatalf("ReadStorage() after write err = %v", err)
	}
	if len(links) != 1 {
		t.Fatalf("len(links) = %d, want 1", len(links))
	}
	if links[0].UUID != link.UUID || links[0].ShortURL != link.ShortURL || links[0].OriginalURL != link.OriginalURL {
		t.Errorf("links[0] = %+v, want %+v", links[0], link)
	}
}

func TestStorage_WriteStorage_Appends(t *testing.T) {
	dir := t.TempDir()
	filename := filepath.Join(dir, "append.json")
	s := NewStorage(filename)

	_ = s.WriteStorage(Link{UUID: 1, ShortURL: "a", OriginalURL: "https://a.com"})
	_ = s.WriteStorage(Link{UUID: 2, ShortURL: "b", OriginalURL: "https://b.com"})

	links, err := s.ReadStorage()
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

func TestStorage_WriteAllStorage(t *testing.T) {
	dir := t.TempDir()
	filename := filepath.Join(dir, "all.json")
	s := NewStorage(filename)

	links := []Link{
		{UUID: 1, ShortURL: "s1", OriginalURL: "https://one.com"},
		{UUID: 2, ShortURL: "s2", OriginalURL: "https://two.com"},
	}
	if err := s.WriteAllStorage(links); err != nil {
		t.Fatalf("WriteAllStorage() err = %v", err)
	}

	read, err := s.ReadStorage()
	if err != nil {
		t.Fatalf("ReadStorage() err = %v", err)
	}
	if len(read) != 2 {
		t.Fatalf("len(read) = %d, want 2", len(read))
	}
	for i := range links {
		if read[i].UUID != links[i].UUID || read[i].ShortURL != links[i].ShortURL || read[i].OriginalURL != links[i].OriginalURL {
			t.Errorf("read[%d] = %+v, want %+v", i, read[i], links[i])
		}
	}
}

func TestStorage_WriteAllStorage_Overwrite(t *testing.T) {
	dir := t.TempDir()
	filename := filepath.Join(dir, "overwrite.json")
	s := NewStorage(filename)
	_ = s.WriteStorage(Link{UUID: 1, ShortURL: "old", OriginalURL: "https://old.com"})

	newLinks := []Link{{UUID: 10, ShortURL: "new", OriginalURL: "https://new.com"}}
	if err := s.WriteAllStorage(newLinks); err != nil {
		t.Fatalf("WriteAllStorage() err = %v", err)
	}

	read, err := s.ReadStorage()
	if err != nil {
		t.Fatalf("ReadStorage() err = %v", err)
	}
	if len(read) != 1 || read[0].ShortURL != "new" {
		t.Errorf("ReadStorage() = %+v, want single link with short_url=new", read)
	}
}

func TestLinkMatchesShortCode(t *testing.T) {
	if LinkMatchesShortCode("http://localhost/abc", "abc") != true {
		t.Fatal("suffix match")
	}
	if LinkMatchesShortCode("abc", "abc") != true {
		t.Fatal("exact match")
	}
	if LinkMatchesShortCode("abc", "") || LinkMatchesShortCode("abc", "xyz") {
		t.Fatal("no match expected")
	}
}

func TestStorage_WriteStorageBatch(t *testing.T) {
	dir := t.TempDir()
	s := NewStorage(filepath.Join(dir, "batch.json"))
	links := []Link{
		{UUID: 1, ShortURL: "s1", OriginalURL: "https://one.com"},
		{UUID: 2, ShortURL: "s2", OriginalURL: "https://two.com"},
	}
	if err := s.WriteStorageBatch(links); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteStorageBatch(nil); err != nil {
		t.Fatal(err)
	}
	read, _ := s.ReadStorage()
	if len(read) != 2 {
		t.Fatalf("len = %d", len(read))
	}
}

func TestStorage_GetShortURLByOriginalURL(t *testing.T) {
	dir := t.TempDir()
	s := NewStorage(filepath.Join(dir, "lookup.json"))
	_ = s.WriteStorage(Link{UUID: 1, ShortURL: "short", OriginalURL: "https://orig.com"})
	got, err := s.GetShortURLByOriginalURL("https://orig.com")
	if err != nil || got != "short" {
		t.Fatalf("got %q, %v", got, err)
	}
	if _, err := s.GetShortURLByOriginalURL("https://missing.com"); err == nil {
		t.Fatal("expected error")
	}
}

func TestStorage_GetLinksByUserID(t *testing.T) {
	dir := t.TempDir()
	s := NewStorage(filepath.Join(dir, "users.json"))
	if links, err := s.GetLinksByUserID(""); err != nil || links != nil {
		t.Fatalf("empty user: %+v, %v", links, err)
	}
	_ = s.WriteStorage(Link{UUID: 1, ShortURL: "a", OriginalURL: "https://a.com", UserID: "u1"})
	links, err := s.GetLinksByUserID("u1")
	if err != nil || len(links) != 1 {
		t.Fatalf("got %+v, %v", links, err)
	}
}

func TestStorage_SoftDeleteURLsByUser(t *testing.T) {
	dir := t.TempDir()
	s := NewStorage(filepath.Join(dir, "del.json"))
	_ = s.WriteStorage(Link{UUID: 1, ShortURL: "http://localhost/c1", OriginalURL: "https://a.com", UserID: "u1"})
	if err := s.SoftDeleteURLsByUser("u1", []string{"c1"}); err != nil {
		t.Fatal(err)
	}
	link, err := s.GetLinkByShortCode("c1")
	if err != nil || !link.IsDeleted {
		t.Fatalf("link = %+v, err = %v", link, err)
	}
}

func TestStorage_GetLinkByShortCode(t *testing.T) {
	dir := t.TempDir()
	s := NewStorage(filepath.Join(dir, "code.json"))
	_ = s.WriteStorage(Link{UUID: 1, ShortURL: "http://localhost/xyz", OriginalURL: "https://a.com"})
	got, err := s.GetLinkByShortCode("xyz")
	if err != nil || got.OriginalURL != "https://a.com" {
		t.Fatalf("got %+v, %v", got, err)
	}
	if _, err := s.GetLinkByShortCode(""); !errors.Is(err, ErrLinkNotFound) {
		t.Fatalf("err = %v", err)
	}
	if _, err := s.GetLinkByShortCode("missing"); !errors.Is(err, ErrLinkNotFound) {
		t.Fatalf("err = %v", err)
	}
}

func TestStorage_NextLinkUUID(t *testing.T) {
	dir := t.TempDir()
	s := NewStorage(filepath.Join(dir, "uuid.json"))
	n, err := s.NextLinkUUID()
	if err != nil || n != 1 {
		t.Fatalf("empty: %d, %v", n, err)
	}
	_ = s.WriteStorage(Link{UUID: 5, ShortURL: "a", OriginalURL: "https://a.com"})
	n, err = s.NextLinkUUID()
	if err != nil || n != 6 {
		t.Fatalf("after write: %d, %v", n, err)
	}
}

func TestStorage_WriteStorage_duplicate(t *testing.T) {
	dir := t.TempDir()
	s := NewStorage(filepath.Join(dir, "dup.json"))
	link := Link{UUID: 1, ShortURL: "a", OriginalURL: "https://dup.com"}
	if err := s.WriteStorage(link); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteStorage(link); !errors.Is(err, ErrDuplicateURL) {
		t.Fatalf("err = %v", err)
	}
}

func TestStorage_ReadStorage_directoryPath(t *testing.T) {
	dir := t.TempDir()
	s := NewStorage(dir)
	if _, err := s.ReadStorage(); err == nil {
		t.Fatal("expected error when storage path is a directory")
	}
}

func TestStorage_WriteStorage_readErrorUsesEmptySlice(t *testing.T) {
	dir := t.TempDir()
	filename := filepath.Join(dir, "corrupt.json")
	if err := os.WriteFile(filename, []byte("not json"), 0644); err != nil {
		t.Fatal(err)
	}
	s := NewStorage(filename)
	link := Link{UUID: 1, ShortURL: "a", OriginalURL: "https://a.com"}
	if err := s.WriteStorage(link); err != nil {
		t.Fatalf("WriteStorage() err = %v", err)
	}
	links, err := s.ReadStorage()
	if err != nil || len(links) != 1 {
		t.Fatalf("links = %+v, err = %v", links, err)
	}
}

func TestShortCodeFromURL(t *testing.T) {
	if got := ShortCodeFromURL("http://localhost/abc"); got != "abc" {
		t.Fatalf("got %q", got)
	}
	if got := ShortCodeFromURL("plain"); got != "plain" {
		t.Fatalf("got %q", got)
	}
}

func TestStorage_WriteStorageBatch_readErrorUsesEmptySlice(t *testing.T) {
	dir := t.TempDir()
	filename := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(filename, []byte("{"), 0644); err != nil {
		t.Fatal(err)
	}
	s := NewStorage(filename)
	links := []Link{{UUID: 1, ShortURL: "a", OriginalURL: "https://a.com"}}
	if err := s.WriteStorageBatch(links); err != nil {
		t.Fatalf("WriteStorageBatch() err = %v", err)
	}
	read, err := s.ReadStorage()
	if err != nil || len(read) != 1 {
		t.Fatalf("read = %+v, err = %v", read, err)
	}
}

func TestStorage_SoftDeleteURLsByUser_alreadyDeleted(t *testing.T) {
	dir := t.TempDir()
	s := NewStorage(filepath.Join(dir, "soft-del.json"))
	_ = s.WriteStorage(Link{UUID: 1, ShortURL: "http://localhost/c1", OriginalURL: "https://a.com", UserID: "u1", IsDeleted: true})
	if err := s.SoftDeleteURLsByUser("u1", []string{"c1"}); err != nil {
		t.Fatal(err)
	}
}

func TestStorage_SoftDeleteURLsByUser_noMatch(t *testing.T) {
	dir := t.TempDir()
	s := NewStorage(filepath.Join(dir, "soft.json"))
	_ = s.WriteStorage(Link{UUID: 1, ShortURL: "http://localhost/c1", OriginalURL: "https://a.com", UserID: "u1"})
	if err := s.SoftDeleteURLsByUser("u1", []string{"missing"}); err != nil {
		t.Fatal(err)
	}
	link, err := s.GetLinkByShortCode("c1")
	if err != nil || link.IsDeleted {
		t.Fatalf("link = %+v, err = %v", link, err)
	}
}

func TestStorage_SoftDeleteURLsByUser_earlyReturn(t *testing.T) {
	dir := t.TempDir()
	s := NewStorage(filepath.Join(dir, "noop.json"))
	if err := s.SoftDeleteURLsByUser("", []string{"x"}); err != nil {
		t.Fatal(err)
	}
	if err := s.SoftDeleteURLsByUser("u1", nil); err != nil {
		t.Fatal(err)
	}
}

func TestStorage_GetLinksByUserID_readError(t *testing.T) {
	dir := t.TempDir()
	filename := filepath.Join(dir, "users-bad.json")
	if err := os.WriteFile(filename, []byte("[]invalid"), 0644); err != nil {
		t.Fatal(err)
	}
	s := NewStorage(filename)
	if _, err := s.GetLinksByUserID("u1"); err == nil {
		t.Fatal("expected read error")
	}
}

func TestStorage_NextLinkUUID_readError(t *testing.T) {
	dir := t.TempDir()
	s := NewStorage(dir)
	n, err := s.NextLinkUUID()
	if err == nil || n != 1 {
		t.Fatalf("NextLinkUUID() = (%d, %v)", n, err)
	}
}

func TestStorage_WriteAllStorage_writeError(t *testing.T) {
	dir := t.TempDir()
	s := NewStorage(dir)
	if err := s.WriteAllStorage([]Link{{UUID: 1, ShortURL: "a", OriginalURL: "https://a.com"}}); err == nil {
		t.Fatal("expected write error when path is a directory")
	}
}

func TestStorage_ReadStorage_cannotCreateFile(t *testing.T) {
	dir := t.TempDir()
	s := NewStorage(filepath.Join(dir, "missing", "data.json"))
	if _, err := s.ReadStorage(); err == nil {
		t.Fatal("expected error when parent directory does not exist")
	}
}

func corruptStorage(t *testing.T) *Storage {
	t.Helper()
	dir := t.TempDir()
	filename := filepath.Join(dir, "corrupt.json")
	if err := os.WriteFile(filename, []byte("{bad"), 0644); err != nil {
		t.Fatal(err)
	}
	return NewStorage(filename)
}

func TestStorage_GetShortURLByOriginalURL_readError(t *testing.T) {
	s := corruptStorage(t)
	if _, err := s.GetShortURLByOriginalURL("https://a.com"); err == nil {
		t.Fatal("expected read error")
	}
}

func TestStorage_SoftDeleteURLsByUser_readError(t *testing.T) {
	s := corruptStorage(t)
	if err := s.SoftDeleteURLsByUser("u1", []string{"c1"}); err == nil {
		t.Fatal("expected read error")
	}
}

func TestStorage_GetLinkByShortCode_readError(t *testing.T) {
	s := corruptStorage(t)
	if _, err := s.GetLinkByShortCode("c1"); err == nil {
		t.Fatal("expected read error")
	}
}

func TestStorage_writeAllStorageLocked_marshalError(t *testing.T) {
	old := marshalStorageLinks
	marshalStorageLinks = func(any, string, string) ([]byte, error) {
		return nil, os.ErrInvalid
	}
	defer func() { marshalStorageLinks = old }()

	s := NewStorage(filepath.Join(t.TempDir(), "marshal.json"))
	if err := s.WriteAllStorage([]Link{{UUID: 1, ShortURL: "a", OriginalURL: "https://a.com"}}); err == nil {
		t.Fatal("expected marshal error")
	}
}

func TestStorage_ReadStorage_createFileError(t *testing.T) {
	old := createStorageFile
	createStorageFile = func(string) error { return os.ErrPermission }
	defer func() { createStorageFile = old }()

	s := NewStorage(filepath.Join(t.TempDir(), "data.json"))
	if _, err := s.ReadStorage(); err == nil {
		t.Fatal("expected error when cannot create storage file")
	}
}
