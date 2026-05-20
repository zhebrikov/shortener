// Package logger настраивает zap и HTTP-middleware для структурированного логирования запросов.
package logger

import (
	"net/http"
	"time"

	"go.uber.org/zap"
)

// New создаёт логгер с заданным уровнем логирования (например "info", "debug").
// Логгер нужно передавать явно в компоненты — так зависимости остаются явными,
// и логгер можно кастомизировать для каждого компонента (например, добавить поле "component").
func New(level string) (*zap.Logger, error) {
	lvl, err := zap.ParseAtomicLevel(level)
	if err != nil {
		return nil, err
	}
	cfg := zap.NewProductionConfig()
	cfg.Level = lvl
	return cfg.Build()
}

// responseWriter оборачивает http.ResponseWriter и сохраняет код статуса и размер ответа.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	written    int64
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.written += int64(n)
	return n, err
}

// Middleware возвращает middleware для chi, логирующий запрос (URI, метод, время) и ответ (код, размер).
// Логгер передаётся явно — зависимость от логгера видна в коде.
func Middleware(log *zap.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
			next.ServeHTTP(wrapped, r)
			duration := time.Since(start)
			log.Info("request completed",
				zap.String("method", r.Method),
				zap.String("uri", r.RequestURI),
				zap.Duration("duration", duration),
				zap.Int("status", wrapped.statusCode),
				zap.Int64("size", wrapped.written),
			)
		})
	}
}

// RequestLogger возвращает middleware, логирующий входящий запрос на уровне Debug.
// Логгер передаётся явно.
func RequestLogger(log *zap.Logger, h http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Debug("got incoming HTTP request",
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
		)
		h(w, r)
	})
}
