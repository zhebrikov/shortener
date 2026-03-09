package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/zhebrikov/shortener/internal/service"
	"github.com/zhebrikov/shortener/internal/storage"
)

// BatchRequestItem — элемент тела запроса POST /api/shorten/batch.
type BatchRequestItem struct {
	CorrelationID string `json:"correlation_id"`
	OriginalURL   string `json:"original_url"`
}

// BatchResponseItem — элемент ответа POST /api/shorten/batch.
type BatchResponseItem struct {
	CorrelationID string `json:"correlation_id"`
	ShortURL      string `json:"short_url"`
}

func CreateLinkBatch(
	w http.ResponseWriter,
	r *http.Request,
	shortener *service.Shortener,
	store storage.LinkStore,
) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	var input []BatchRequestItem
	if err = json.Unmarshal(body, &input); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if len(input) == 0 {
		http.Error(w, "empty batch not allowed", http.StatusBadRequest)
		return
	}

	links, err := store.ReadStorage()
	if err != nil {
		http.Error(w, "Failed to read storage", http.StatusInternalServerError)
		return
	}

	existingOriginalToShort := make(map[string]string)
	var maxUUID int
	for _, link := range links {
		existingOriginalToShort[link.OriginalURL] = link.ShortURL
		if link.UUID > maxUUID {
			maxUUID = link.UUID
		}
	}

	var toWrite []storage.Link
	response := make([]BatchResponseItem, 0, len(input))
	seenInBatch := make(map[string]string)

	for _, item := range input {
		originalURL := strings.TrimSpace(item.OriginalURL)
		if originalURL == "" {
			http.Error(w, "original_url must be non-empty", http.StatusBadRequest)
			return
		}

		var shortURL string
		if s, ok := existingOriginalToShort[originalURL]; ok {
			shortURL = s
		} else if s, ok := seenInBatch[originalURL]; ok {
			shortURL = s
		} else {
			var createErr error
			shortURL, createErr = shortener.CreateLink(originalURL)
			if createErr != nil {
				http.Error(w, createErr.Error(), http.StatusInternalServerError)
				return
			}
			seenInBatch[originalURL] = shortURL
			maxUUID++
			toWrite = append(toWrite, storage.Link{
				UUID:          maxUUID,
				ShortURL:      shortURL,
				OriginalURL:   originalURL,
				CorrelationID: item.CorrelationID,
			})
			existingOriginalToShort[originalURL] = shortURL
		}

		response = append(response, BatchResponseItem{
			CorrelationID: item.CorrelationID,
			ShortURL:      shortURL,
		})
	}

	if len(toWrite) > 0 {
		if err = store.WriteStorageBatch(toWrite); err != nil {
			http.Error(w, "Failed to write storage", http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(response)
}
