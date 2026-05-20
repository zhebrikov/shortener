// Package handler реализует HTTP-обработчики сервиса сокращения ссылок.
package handler

import (
	"net/http"

	"github.com/zhebrikov/shortener/internal/asyncdelete"
	"github.com/zhebrikov/shortener/internal/audit"
	"github.com/zhebrikov/shortener/internal/service"
	"github.com/zhebrikov/shortener/internal/storage"
)

// ShortenerHandler группирует зависимости и делегирует им обработку HTTP-запросов.
type ShortenerHandler struct {
	shortener *service.Shortener
	storage   storage.LinkStore
	deleter   *asyncdelete.Worker
	audit     *audit.Publisher
}

// NewShortenerHandler создаёт обработчик с заданными сервисом, хранилищем, воркером удаления и аудитом.
func NewShortenerHandler(shortener *service.Shortener, store storage.LinkStore, deleter *asyncdelete.Worker, auditPub *audit.Publisher) *ShortenerHandler {
	return &ShortenerHandler{
		shortener: shortener,
		storage:   store,
		deleter:   deleter,
		audit:     auditPub,
	}
}

// CreateLink обрабатывает POST / — сокращение URL из тела text/plain.
func (h *ShortenerHandler) CreateLink(w http.ResponseWriter, r *http.Request) {
	CreateLink(w, r, h.shortener, h.storage, h.audit)
}

// GetLink обрабатывает GET /{shortCode} — редирект на оригинальный URL.
func (h *ShortenerHandler) GetLink(w http.ResponseWriter, r *http.Request) {
	GetLink(w, r, h.shortener, h.storage, h.audit)
}

// CreateLinkJSON обрабатывает POST /api/shorten — сокращение URL из JSON.
func (h *ShortenerHandler) CreateLinkJSON(w http.ResponseWriter, r *http.Request) {
	CreateLinkJSON(w, r, h.shortener, h.storage, h.audit)
}

// CreateLinkBatch обрабатывает POST /api/shorten/batch — пакетное сокращение.
func (h *ShortenerHandler) CreateLinkBatch(w http.ResponseWriter, r *http.Request) {
	CreateLinkBatch(w, r, h.shortener, h.storage)
}

// DeleteUserURLs обрабатывает DELETE /api/user/urls — асинхронное удаление ссылок пользователя.
func (h *ShortenerHandler) DeleteUserURLs(w http.ResponseWriter, r *http.Request) {
	DeleteUserURLs(w, r, h.deleter)
}
