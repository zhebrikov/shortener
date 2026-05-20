package handler

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"

	"github.com/zhebrikov/shortener/internal/audit"
	"github.com/zhebrikov/shortener/internal/auth"
	"github.com/zhebrikov/shortener/internal/service"
	"github.com/zhebrikov/shortener/internal/storage"
)

// Input — тело запроса POST /api/shorten.
type Input struct {
	URL string `json:"url"`
}

// Output — ответ POST /api/shorten.
type Output struct {
	Result string `json:"result"`
}

// CreateLinkJSON сокращает URL из JSON-тела запроса и сохраняет запись в хранилище.
func CreateLinkJSON(
	w http.ResponseWriter,
	r *http.Request,
	shortener *service.Shortener,
	store storage.LinkStore,
	auditPub *audit.Publisher,
) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	var input Input
	err = json.Unmarshal(body, &input)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	shortURL, err := shortener.CreateLink(input.URL)
	if err != nil {
		log.Printf("CreateLinkJSON: shortener.CreateLink: %v", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	nextUUID, err := store.NextLinkUUID()
	if err != nil {
		log.Printf("CreateLinkJSON: store.NextLinkUUID: %v", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	userID, _ := auth.UserIDFromContext(r.Context())
	newRecord := storage.Link{
		UUID:        nextUUID,
		ShortURL:    shortURL,
		OriginalURL: input.URL,
		UserID:      userID,
	}

	err = store.WriteStorage(newRecord)
	if err != nil {
		if errors.Is(err, storage.ErrDuplicateURL) {
			existingShort, getErr := store.GetShortURLByOriginalURL(input.URL)
			if getErr != nil {
				log.Printf("CreateLinkJSON: store.GetShortURLByOriginalURL: %v", getErr)
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			resultJSON, _ := json.Marshal(Output{Result: existingShort})
			w.Write(resultJSON)
			return
		}
		log.Printf("CreateLinkJSON: store.WriteStorage: %v", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	publishAudit(auditPub, r, audit.ActionShorten, input.URL)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(Output{Result: shortURL})
}
