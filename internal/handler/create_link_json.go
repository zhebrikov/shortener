package handler

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
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
func CreateLinkJSON(w http.ResponseWriter, r *http.Request, h *ShortenerHandler) {
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

	result, err := h.app.ShortenURL(r.Context(), input.URL)
	if err != nil {
		log.Printf("CreateLinkJSON: ShortenURL: %v", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if result.IsConflict {
		w.WriteHeader(http.StatusConflict)
	} else {
		w.WriteHeader(http.StatusCreated)
	}
	_ = json.NewEncoder(w).Encode(Output{Result: result.ShortURL})
}
