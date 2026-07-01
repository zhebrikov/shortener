package handler

import (
	"encoding/json"
	"net/http"

	"github.com/zhebrikov/shortener/internal/middleware"
	"github.com/zhebrikov/shortener/internal/storage"
)

type internalStatsResponse struct {
	URLs  int `json:"urls"`
	Users int `json:"users"`
}

// InternalStats возвращает статистику сервиса; доступ только из доверенной подсети (заголовок X-Real-IP).
func InternalStats(trustedSubnet string, store storage.LinkStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !middleware.IPInTrustedSubnet(trustedSubnet, r.Header.Get("X-Real-IP")) {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		urls, err := store.CountURLs()
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		users, err := store.CountUsers()
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
