package handler

import (
	"net/http"

	"github.com/zhebrikov/shortener/internal/service"
	"github.com/zhebrikov/shortener/internal/storage"
)

type ShortenerHandler struct {
	shortener *service.Shortener
	storage   storage.LinkStore
}

func NewShortenerHandler(shortener *service.Shortener, store storage.LinkStore) *ShortenerHandler {
	return &ShortenerHandler{
		shortener: shortener,
		storage:   store,
	}
}

func (h *ShortenerHandler) CreateLink(w http.ResponseWriter, r *http.Request) {
	CreateLink(w, r, h.shortener, h.storage)
}

func (h *ShortenerHandler) GetLink(w http.ResponseWriter, r *http.Request) {
	GetLink(w, r, h.shortener, h.storage)
}

func (h *ShortenerHandler) CreateLinkJSON(w http.ResponseWriter, r *http.Request) {
	CreateLinkJSON(w, r, h.shortener, h.storage)
}

func (h *ShortenerHandler) CreateLinkBatch(w http.ResponseWriter, r *http.Request) {
	CreateLinkBatch(w, r, h.shortener, h.storage)
}
