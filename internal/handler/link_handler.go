package handler

import (
	"net/http"

	"github.com/zhebrikov/shortener/internal/service"
)

type ShortenerHandler struct {
	shortener *service.Shortener
}

func NewShortenerHandler(shortener *service.Shortener) *ShortenerHandler {
	return &ShortenerHandler{
		shortener: shortener,
	}
}

func (h *ShortenerHandler) CreateLink(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		CreateLink(w, r, h.shortener)
		return
	}

	if r.Method == http.MethodGet {
		GetLink(w, r, h.shortener)
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func (h *ShortenerHandler) GetLink(w http.ResponseWriter, r *http.Request) {
	GetLink(w, r, h.shortener)
}

func (h *ShortenerHandler) CreateLinkJson(w http.ResponseWriter, r *http.Request) {
	CreateLinkJson(w, r, h.shortener)
}
