package handler

import (
	"errors"
	"io"
	"net/http"

	"github.com/zhebrikov/shortener/internal/audit"
	"github.com/zhebrikov/shortener/internal/auth"
	"github.com/zhebrikov/shortener/internal/service"
	"github.com/zhebrikov/shortener/internal/storage"
)

// CreateLink сокращает URL из тела запроса (Content-Type: text/plain) и сохраняет запись в хранилище.
func CreateLink(w http.ResponseWriter, r *http.Request, shortener *service.Shortener, store storage.LinkStore, auditPub *audit.Publisher) {
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

	shortURL, err := shortener.CreateLink(originalURL)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	nextUUID, err := storage.NewLinkUUID()
	if err != nil {
		http.Error(w, "Failed to generate link id", http.StatusInternalServerError)
		return
	}

	userID, _ := auth.UserIDFromContext(r.Context())
	newRecord := storage.Link{
		UUID:        nextUUID,
		ShortURL:    shortURL,
		OriginalURL: originalURL,
		UserID:      userID,
	}

	if err := store.WriteStorage(newRecord); err != nil {
		if errors.Is(err, storage.ErrDuplicateURL) {
			existingShort, getErr := store.GetShortURLByOriginalURL(originalURL)
			if getErr != nil {
				http.Error(w, "Failed to get existing short URL", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusConflict)
			w.Write([]byte(existingShort))
			return
		}
		http.Error(w, "Failed to write storage", http.StatusInternalServerError)
		return
	}

	publishAudit(auditPub, r, audit.ActionShorten, originalURL)
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(shortURL))
}
