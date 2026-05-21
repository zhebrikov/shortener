package handler

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/zhebrikov/shortener/internal/auth"
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

// CreateLinkBatch сокращает несколько URL за один запрос (POST /api/shorten/batch).
func CreateLinkBatch(
	w http.ResponseWriter,
	r *http.Request,
	shortener *service.Shortener,
	store storage.LinkStore,
) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("CreateLinkBatch: io.ReadAll: %v", err)
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	var input []BatchRequestItem
	if err = json.Unmarshal(body, &input); err != nil {
		log.Printf("CreateLinkBatch: json.Unmarshal: %v", err)
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	if len(input) == 0 {
		log.Printf("CreateLinkBatch: empty batch input")
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	response := make([]BatchResponseItem, 0, len(input))
	seenInBatch := make(map[string]string)
	userID, _ := auth.UserIDFromContext(r.Context())

	for _, item := range input {
		originalURL := strings.TrimSpace(item.OriginalURL)
		if originalURL == "" {
			log.Printf("CreateLinkBatch: empty original_url for correlation_id=%q", item.CorrelationID)
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		var shortURL string
		if s, ok := seenInBatch[originalURL]; ok {
			shortURL = s
		} else {
			shortURL, err = shortener.CreateLink(originalURL)
			if err != nil {
				log.Printf("CreateLinkBatch: shortener.CreateLink: %v", err)
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
			linkUUID, uuidErr := storage.NewLinkUUID()
			if uuidErr != nil {
				log.Printf("CreateLinkBatch: storage.NewLinkUUID: %v", uuidErr)
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
			newRecord := storage.Link{
				UUID:        linkUUID,
				ShortURL:    shortURL,
				OriginalURL: originalURL,
				UserID:      userID,
			}
			err = store.WriteStorage(newRecord)
			if err != nil {
				if errors.Is(err, storage.ErrDuplicateURL) {
					shortURL, err = store.GetShortURLByOriginalURL(originalURL)
					if err != nil {
						log.Printf("CreateLinkBatch: GetShortURLByOriginalURL: %v", err)
						http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
						return
					}
				} else {
					log.Printf("CreateLinkBatch: store.WriteStorage: %v", err)
					http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
					return
				}
			}
			seenInBatch[originalURL] = shortURL
		}

		response = append(response, BatchResponseItem{
			CorrelationID: item.CorrelationID,
			ShortURL:      shortURL,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(response)
}
