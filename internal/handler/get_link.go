package handler

import (
	"net/http"

	"github.com/zhebrikov/shortener/internal/service"
)

func GetLink(w http.ResponseWriter, r *http.Request, shortener *service.Shortener) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	shortCode := r.URL.Path[1:]

	originalURL := shortener.GetLink(shortCode)

	if originalURL == nil {
		http.NotFound(w, r)
		return
	}

	// Редирект 307
	w.Header().Set("Location", *originalURL)
	w.WriteHeader(http.StatusTemporaryRedirect)
}
