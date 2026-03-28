package storage

import (
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
