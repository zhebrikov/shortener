package handler

import (
	"encoding/json"
	"net/http"

	"github.com/zhebrikov/shortener/internal/storage"
)

type internalStatsResponse struct {
	URLs  int `json:"urls"`
	Users int `json:"users"`
}

// InternalStats возвращает статистику сервиса.
func InternalStats(store storage.LinkStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		urls, users, err := store.Stats()
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(internalStatsResponse{URLs: urls, Users: users}); err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
	}
}
