package handler

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/zhebrikov/shortener/internal/service"
)

type Input struct {
	URL string `json:"url"`
}

type Output struct {
	Result string `json:"result"`
}

func CreateLinkJson(w http.ResponseWriter, r *http.Request, shortener *service.Shortener) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

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

	shortURL := shortener.CreateLink(input.URL)

	w.Header().Set("Content-Type", "application/json")
	result := Output{Result: shortURL}
	resultJson, err := json.Marshal(result)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusCreated)
	w.Write(resultJson)
}
