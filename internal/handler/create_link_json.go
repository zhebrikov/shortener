package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/zhebrikov/shortener/internal/service"
	"github.com/zhebrikov/shortener/internal/storage"
)

type Input struct {
	URL string `json:"url"`
}

type Output struct {
	Result string `json:"result"`
}

func CreateLinkJSON(
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

	var input Input
	err = json.Unmarshal(body, &input)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	shortURL, err := shortener.CreateLink(input.URL)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	links, err := store.ReadStorage()
	if err != nil {
		http.Error(w, "Failed to read storage", http.StatusInternalServerError)
		return
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
		OriginalURL: input.URL,
	}

	err = store.WriteStorage(newRecord)
	if err != nil {
		if errors.Is(err, storage.ErrDuplicateURL) {
			existingShort, getErr := store.GetShortURLByOriginalURL(input.URL)
			if getErr != nil {
				http.Error(w, "Failed to get existing short URL", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			resultJSON, _ := json.Marshal(Output{Result: existingShort})
			w.Write(resultJSON)
			return
		}
		http.Error(w, "Failed to write storage", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	result := Output{Result: shortURL}
	resultJSON, err := json.Marshal(result)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusCreated)
	w.Write(resultJSON)
}
