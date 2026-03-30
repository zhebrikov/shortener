package handler

import (
	"net/http"

	"github.com/zhebrikov/shortener/internal/asyncdelete"
	"github.com/zhebrikov/shortener/internal/service"
	"github.com/zhebrikov/shortener/internal/storage"
)

type ShortenerHandler struct {
	shortener *service.Shortener
	storage   storage.LinkStore
	deleter   *asyncdelete.Worker
}

func NewShortenerHandler(shortener *service.Shortener, store storage.LinkStore, deleter *asyncdelete.Worker) *ShortenerHandler {
	return &ShortenerHandler{
		shortener: shortener,
		storage:   store,
		deleter:   deleter,
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

func (h *ShortenerHandler) ListUserURLs(w http.ResponseWriter, r *http.Request) {
	ListUserURLs(w, r, h.storage)
}

func (h *ShortenerHandler) DeleteUserURLs(w http.ResponseWriter, r *http.Request) {
	DeleteUserURLs(w, r, h.deleter)
}
