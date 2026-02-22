package handler

import (
	"io"
	"net/http"

	"github.com/zhebrikov/shortener/internal/service"
	"github.com/zhebrikov/shortener/internal/storage"
)

func CreateLink(w http.ResponseWriter, r *http.Request, shortener *service.Shortener, store *storage.Storage) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if r.Header.Get("Content-Type") != "text/plain" {
		http.Error(w, "Content-Type must be text/plain", http.StatusBadRequest)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil || len(body) == 0 {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	originalURL := string(body)

	shortURL := shortener.CreateLink(originalURL)

	links, err := store.ReadStorage()
	if err != nil {
		http.Error(w, "Failed to read storage", http.StatusInternalServerError)
		return
	}

	for _, link := range links {
		if link.OriginalURL == originalURL {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusConflict)
			w.Write([]byte(link.ShortURL))
			return
		}
	}

	var lastLinkUUID int
	if len(links) == 0 {
		lastLinkUUID = 0
	} else {
		lastLinkUUID = links[len(links)-1].UUID
	}

	newRecord := storage.Link{
		UUID:        lastLinkUUID + 1,
		ShortURL:    shortURL,
		OriginalURL: originalURL,
	}

	if err := store.WriteStorage(newRecord); err != nil {
		http.Error(w, "Failed to write storage", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(shortURL))
}
