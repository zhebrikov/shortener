package handler

import (
	"encoding/json"
	"net/http"

	"github.com/zhebrikov/shortener/internal/asyncdelete"
	"github.com/zhebrikov/shortener/internal/auth"
)

// DeleteUserURLs принимает JSON-массив идентификаторов сокращённых URL и ставит их на асинхронное удаление (202 Accepted).
func DeleteUserURLs(w http.ResponseWriter, r *http.Request, wkr *asyncdelete.Worker) {
	if auth.EmptyAuthCookieFromContext(r.Context()) {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	var ids []string
	if err := json.NewDecoder(r.Body).Decode(&ids); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if wkr != nil {
		wkr.Submit(userID, ids)
	}
	w.WriteHeader(http.StatusAccepted)
}
