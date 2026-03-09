package handler

import (
	"database/sql"
	"net/http"
)

// HealthCheck возвращает http.HandlerFunc для проверки доступности БД.
// Если db == nil (хранилище не БД), всегда возвращает 200 OK.
func HealthCheck(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if db == nil {
			w.WriteHeader(http.StatusOK)
			return
		}
		if err := db.Ping(); err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}
