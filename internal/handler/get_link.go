package handler

import (
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

	originalURL, gone := shortener.GetLink(shortCode, store)

	if gone {
		w.WriteHeader(http.StatusGone)
		return
	}
	if originalURL == nil {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Location", *originalURL)
	w.WriteHeader(http.StatusTemporaryRedirect)
}
