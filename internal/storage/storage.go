package storage

import (
	"encoding/json"
	"log"
	"os"
)

type Storage struct {
	filename string
}

type Link struct {
	UUID        int    `json:"uuid"`
	ShortURL    string `json:"short_url"`
	OriginalURL string `json:"original_url"`
}

func NewStorage(filename string) *Storage {
	return &Storage{filename: filename}
}

func (s *Storage) ReadStorage() ([]Link, error) {
	data, err := os.ReadFile(s.filename)
	if err != nil {
		if os.IsNotExist(err) {
			if err := os.WriteFile(s.filename, []byte("[]"), 0644); err != nil {
				return nil, err
			}
			return []Link{}, nil
		}
		log.Println("Error reading storage", err)
		return nil, err
	}
	var storage []Link
	if err = json.Unmarshal(data, &storage); err != nil {
		return nil, err
	}
	return storage, nil
}

func (s *Storage) WriteStorage(link Link) error {
	links, err := s.ReadStorage()
	if err != nil {
		links = []Link{}
	}
	links = append(links, link)
	return s.WriteAllStorage(links)
}

// WriteAllStorage перезаписывает файл полным списком ссылок (JSON-массив).
func (s *Storage) WriteAllStorage(links []Link) error {
	data, err := json.MarshalIndent(links, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.filename, data, 0644)
}
