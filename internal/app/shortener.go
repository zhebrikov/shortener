// Package app содержит транспортно-независимую бизнес-логику сервиса сокращения ссылок.
package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/zhebrikov/shortener/internal/audit"
	"github.com/zhebrikov/shortener/internal/auth"
	"github.com/zhebrikov/shortener/internal/service"
	"github.com/zhebrikov/shortener/internal/storage"
)

// UserURL — пара сокращённого и оригинального URL пользователя.
type UserURL struct {
	ShortURL    string
	OriginalURL string
}

// ShortenResult — результат сокращения URL.
type ShortenResult struct {
	ShortURL   string
	IsConflict bool
}

var (
	// ErrUnauthorized — запрос без валидной авторизации.
	ErrUnauthorized = errors.New("unauthorized")
	// ErrEmptyAuth — передан пустой authorization.
	ErrEmptyAuth = errors.New("empty authorization")
)

// ShortenerApp инкапсулирует сценарии использования сервиса.
type ShortenerApp struct {
	shortener *service.Shortener
	store     storage.LinkStore
	audit     *audit.Publisher
}

// NewShortenerApp создаёт фасад бизнес-логики.
func NewShortenerApp(shortener *service.Shortener, store storage.LinkStore, auditPub *audit.Publisher) *ShortenerApp {
	return &ShortenerApp{
		shortener: shortener,
		store:     store,
		audit:     auditPub,
	}
}

// ShortenURL сокращает URL и сохраняет запись в хранилище.
func (a *ShortenerApp) ShortenURL(ctx context.Context, originalURL string) (ShortenResult, error) {
	shortURL, err := a.shortener.CreateLink(originalURL)
	if err != nil {
		return ShortenResult{}, fmt.Errorf("create link: %w", err)
	}

	nextUUID, err := storage.NewLinkUUID()
	if err != nil {
		return ShortenResult{}, fmt.Errorf("generate link id: %w", err)
	}

	userID, _ := auth.UserIDFromContext(ctx)
	newRecord := storage.Link{
		UUID:        nextUUID,
		ShortURL:    shortURL,
		OriginalURL: originalURL,
		UserID:      userID,
	}

	if err := a.store.WriteStorage(newRecord); err != nil {
		if errors.Is(err, storage.ErrDuplicateURL) {
			existingShort, getErr := a.store.GetShortURLByOriginalURL(originalURL)
			if getErr != nil {
				return ShortenResult{}, fmt.Errorf("get existing short url: %w", getErr)
			}
			return ShortenResult{ShortURL: existingShort, IsConflict: true}, nil
		}
		return ShortenResult{}, fmt.Errorf("write storage: %w", err)
	}

	publishAudit(a.audit, ctx, audit.ActionShorten, originalURL)
	return ShortenResult{ShortURL: shortURL}, nil
}

// ExpandURL возвращает оригинальный URL по shortCode.
func (a *ShortenerApp) ExpandURL(ctx context.Context, shortCode string) (string, error) {
	originalURL, err := a.shortener.GetLink(shortCode, a.store)
	if err != nil {
		return "", err
	}
	publishAudit(a.audit, ctx, audit.ActionFollow, originalURL)
	return originalURL, nil
}

// ListUserURLs возвращает все сокращённые URL текущего пользователя.
func (a *ShortenerApp) ListUserURLs(ctx context.Context) ([]UserURL, error) {
	if auth.EmptyAuthCookieFromContext(ctx) {
		return nil, ErrEmptyAuth
	}
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, ErrUnauthorized
	}

	links, err := a.store.GetLinksByUserID(userID)
	if err != nil {
		return nil, fmt.Errorf("get links by user: %w", err)
	}

	out := make([]UserURL, 0, len(links))
	for _, l := range links {
		out = append(out, UserURL{ShortURL: l.ShortURL, OriginalURL: l.OriginalURL})
	}
	return out, nil
}

func publishAudit(p *audit.Publisher, ctx context.Context, action, rawURL string) {
	if p == nil {
		return
	}
	ev := audit.Event{TS: time.Now().Unix(), Action: action, URL: rawURL}
	if uid, ok := auth.UserIDFromContext(ctx); ok {
		ev.UserID = uid
	}
	p.Publish(ev)
}
