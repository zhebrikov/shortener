package handler

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/zhebrikov/shortener/internal/app"
)

// UserURLItem — пара сокращённого и оригинального URL в ответе GET /api/user/urls.
type UserURLItem struct {
	ShortURL    string `json:"short_url"`
	OriginalURL string `json:"original_url"`
}

// ListUserURLs возвращает все сокращённые пользователем URL.
func ListUserURLs(w http.ResponseWriter, r *http.Request, h *ShortenerHandler) {
	links, err := h.app.ListUserURLs(r.Context())
	if errors.Is(err, app.ErrUnauthorized) || errors.Is(err, app.ErrEmptyAuth) {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	if err != nil {
		log.Printf("ListUserURLs: ListUserURLs: %v", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if len(links) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	out := make([]UserURLItem, 0, len(links))
	for _, l := range links {
		out = append(out, UserURLItem{ShortURL: l.ShortURL, OriginalURL: l.OriginalURL})
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(out); err != nil {
		log.Printf("ListUserURLs: encode: %v", err)
	}
}
