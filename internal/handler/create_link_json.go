package handler

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/zhebrikov/shortener/internal/service"
	"github.com/zhebrikov/shortener/internal/storage"
)

type Input struct {
	URL string `json:"url"`
}

func CreateLinkJson(
	w http.ResponseWriter,
	r *http.Request,
	shortener *service.Shortener,
	store *storage.Storage,
) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

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

	shortURL := shortener.CreateLink(input.URL)

	links, err := store.ReadStorage()
	if err != nil {
		http.Error(w, "Failed to read storage", http.StatusInternalServerError)
		return
	}
	for _, link := range links {
		if link.OriginalURL == input.URL {
			w.Header().Set("Content-Type", "application/json")
			result := Input{URL: link.ShortURL}
			resultJson, err := json.Marshal(result)
			if err != nil {
				http.Error(w, "Invalid request body", http.StatusBadRequest)
				return
			}
			w.WriteHeader(http.StatusCreated)
			w.Write(resultJson)
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
		OriginalURL: input.URL,
	}

	err = store.WriteStorage(newRecord)
	if err != nil {
		http.Error(w, "Failed to write storage", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	result := Input{URL: shortURL}
	resultJson, err := json.Marshal(result)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusCreated)
	w.Write(resultJson)
}
