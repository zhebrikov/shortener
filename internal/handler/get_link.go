package handler

import (
	"errors"
	"net/http"

	"github.com/zhebrikov/shortener/internal/service"
	"github.com/zhebrikov/shortener/internal/storage"
)

func GetLink(
	w http.ResponseWriter,
	r *http.Request,
	shortener *service.Shortener,
	store storage.LinkStore,
) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	shortCode := r.URL.Path[1:]

	originalURL, err := shortener.GetLink(shortCode, store)

	if errors.Is(err, service.ErrLinkDeleted) {
		w.WriteHeader(http.StatusGone)
		return
	}

	if errors.Is(err, service.ErrLinkNotFound) {
		http.NotFound(w, r)
		return
	}

	if err != nil {
		http.Error(w, "Failed to get link", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Location", originalURL)
	w.WriteHeader(http.StatusTemporaryRedirect)
}
