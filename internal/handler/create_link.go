package handler

import (
	"io"
	"net/http"
)

// CreateLink сокращает URL из тела запроса (Content-Type: text/plain) и сохраняет запись в хранилище.
func CreateLink(w http.ResponseWriter, r *http.Request, h *ShortenerHandler) {
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

	result, err := h.app.ShortenURL(r.Context(), originalURL)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	if result.IsConflict {
		w.WriteHeader(http.StatusConflict)
	} else {
		w.WriteHeader(http.StatusCreated)
	}
	w.Write([]byte(result.ShortURL))
}
